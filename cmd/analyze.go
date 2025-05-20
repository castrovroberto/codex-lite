package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/castrovroberto/codex-lite/internal/agents"
	"github.com/castrovroberto/codex-lite/internal/contextkeys"
	"github.com/castrovroberto/codex-lite/internal/ollama"
	"github.com/castrovroberto/codex-lite/internal/orchestrator"
	"github.com/castrovroberto/codex-lite/internal/scanner"
	"github.com/castrovroberto/codex-lite/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

var (
	agentNames []string
	recursive  bool
	maxDepth   int
	ignoreDirs []string
	extensions []string
	noTui      bool

	analyzeCmd = &cobra.Command{
		Use:   "analyze [file patterns...]",
		Short: "Analyze code files using specified agents",
		Long: `Analyze command processes specified code files or patterns using a suite of agents.
You can specify which agents to run using the --agent flag (comma-separated).
If no agents are specified, all available agents will be run.
Use --recursive to scan directories recursively.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			appCfg := contextkeys.ConfigFromContext(cmd.Context())
			log := contextkeys.LoggerFromContext(cmd.Context())

			if log == nil {
				fmt.Fprintln(os.Stderr, "Error: Logger not found in context. Using a temporary basic logger.")
				log = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
			}

			if appCfg.OllamaHostURL == "" {
				log.Error("Critical configuration (OllamaHost) is missing or empty.")
				fmt.Fprintln(os.Stderr, `Error: Ollama host URL is not configured. To fix this, you can:

1. Set the CODEXLITE_OLLAMA_HOST_URL environment variable:
   export CODEXLITE_OLLAMA_HOST_URL="http://localhost:11434"

2. Add to your config file (~/.codex-lite.yaml or .codex-lite.yaml):
   ollama_host_url: "http://localhost:11434"

3. Use the --ollama-host-url flag:
   codex-lite analyze --ollama-host-url="http://localhost:11434" [files...]

Make sure Ollama is running and accessible at the configured URL.
Default Ollama URL is usually: http://localhost:11434`)
				return errors.New("ollama host URL not configured")
			}

			if appCfg.DefaultModel == "" {
				log.Warn("No default model configured, will use 'llama2'")
				fmt.Fprintln(os.Stderr, `Warning: No default model configured. Using 'llama2'.
To set a different model, you can:

1. Set the CODEXLITE_DEFAULT_MODEL environment variable:
   export CODEXLITE_DEFAULT_MODEL="your-model-name"

2. Add to your config file (~/.codex-lite.yaml or .codex-lite.yaml):
   default_model: "your-model-name"

3. Use the --default-model flag:
   codex-lite analyze --default-model="your-model-name" [files...]

Available models depend on your Ollama installation.`)
			}

			log.Info("Codex Lite analyze command starting...")
			log.Debug("Loaded configuration from context", "ollama_host", appCfg.OllamaHostURL, "default_model", appCfg.DefaultModel)

			// Setup context that can be cancelled by SIGINT or when analysis completes.
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop() // Important to release resources associated with NotifyContext

			// This function contains the core analysis logic
			// It will be run in a goroutine if TUI is enabled, or directly if not.
			analyzeLogicFunc := func(analysisCtx context.Context, tuiProgram *tea.Program, tuiModel *tui.Model) error {
				// Ensure that if this function finishes (normally or error), it tries to quit the TUI
				if tuiProgram != nil {
					defer tuiProgram.Quit()
				}

				agentOrchestrator := orchestrator.NewBasicOrchestrator()
				if err := agentOrchestrator.RegisterAgent(agents.NewExplainAgent()); err != nil {
					log.Warn("Failed to register Explain agent", "error", err)
				}
				if err := agentOrchestrator.RegisterAgent(agents.NewSyntaxAgent()); err != nil {
					log.Warn("Failed to register Syntax agent", "error", err)
				}
				if err := agentOrchestrator.RegisterAgent(agents.NewSmellAgent()); err != nil {
					log.Warn("Failed to register Smell agent", "error", err)
				}
				if err := agentOrchestrator.RegisterAgent(agents.NewSecurityAgent()); err != nil {
					log.Warn("Failed to register Security agent", "error", err)
				}
				if err := agentOrchestrator.RegisterAgent(agents.NewAdvancedAgent(appCfg.WorkspaceRoot)); err != nil {
					log.Warn("Failed to register Advanced agent", "error", err)
				}

				var agentsToRun []string
				if len(agentNames) == 0 {
					log.Info("No specific agents requested, attempting to run all registered agents.")
					agentsToRun = agentOrchestrator.AvailableAgentNames()
				} else {
					for _, requestedName := range agentNames {
						if _, err := agentOrchestrator.GetAgent(requestedName); err != nil {
							log.Warn("Requested agent not found, will be skipped.", "agent_name", requestedName)
							fmt.Printf("⚠️ Warning: Agent '%s' not found. Skipping.\n", requestedName)
						} else {
							agentsToRun = append(agentsToRun, requestedName)
						}
					}
				}

				if len(agentsToRun) == 0 {
					log.Info("No valid agents selected or available to run. Exiting.")
					fmt.Println("No valid agents selected or available to run.")
					return nil
				}
				log.Info("Selected agents to run", "agents", agentsToRun)

				if len(args) == 0 {
					log.Error("No file patterns specified.")
					fmt.Println("Error: No file patterns specified.")
					return errors.New("no file patterns specified")
				}

				scannerOpts := scanner.DefaultOptions()
				if maxDepth >= 0 {
					scannerOpts.MaxDepth = maxDepth
				}
				if len(ignoreDirs) > 0 {
					customIgnoreDirs := make(map[string]bool)
					for _, dir := range ignoreDirs {
						customIgnoreDirs[dir] = true
					}
					scannerOpts.IgnoreDirs = customIgnoreDirs
				}
				if len(extensions) > 0 {
					customExtensions := make(map[string]bool)
					for _, ext := range extensions {
						if !strings.HasPrefix(ext, ".") {
							ext = "." + ext
						}
						customExtensions[ext] = true
					}
					scannerOpts.SourceExtensions = customExtensions
				}
				codeScanner := scanner.NewScanner(scannerOpts)
				var filesToAnalyze []string
				for _, pattern := range args {
					if recursive {
						info, err := os.Stat(pattern)
						if err == nil && info.IsDir() {
							results, err := codeScanner.Scan(pattern)
							if err != nil {
								log.Error("Error scanning directory", "directory", pattern, "error", err)
								continue
							}
							for _, result := range results {
								filesToAnalyze = append(filesToAnalyze, result.Path)
							}
						} else {
							matches, err := filepath.Glob(pattern)
							if err != nil {
								log.Error("Invalid file pattern", "pattern", pattern, "error", err)
								continue
							}
							for _, match := range matches {
								if info, err := os.Stat(match); err == nil {
									if info.IsDir() {
										results, err := codeScanner.Scan(match)
										if err != nil {
											log.Error("Error scanning directory", "directory", match, "error", err)
											continue
										}
										for _, result := range results {
											filesToAnalyze = append(filesToAnalyze, result.Path)
										}
									} else if codeScanner.IsSourceFile(match) {
										filesToAnalyze = append(filesToAnalyze, match)
									}
								}
							}
						}
					} else {
						matches, err := filepath.Glob(pattern)
						if err != nil {
							log.Error("Invalid file pattern", "pattern", pattern, "error", err)
							continue
						}
						for _, match := range matches {
							if info, err := os.Stat(match); err == nil && !info.IsDir() && codeScanner.IsSourceFile(match) {
								filesToAnalyze = append(filesToAnalyze, match)
							}
						}
					}
				}
				if len(filesToAnalyze) == 0 {
					log.Info("No files found matching the pattern(s) or specified paths after filtering. Exiting.")
					fmt.Println("No files found to analyze.")
					return nil
				}
				seen := make(map[string]bool)
				uniqueFiles := []string{}
				for _, file := range filesToAnalyze {
					absPath, err := filepath.Abs(file)
					if err != nil {
						log.Warn("Could not get absolute path for file, using as is", "file", file, "error", err)
						absPath = file
					}
					if !seen[absPath] {
						seen[absPath] = true
						uniqueFiles = append(uniqueFiles, absPath)
					}
				}
				filesToAnalyze = uniqueFiles
				log.Info("Files to analyze", "count", len(filesToAnalyze))

				if tuiModel != nil {
					tuiModel.StartProcessing() // Signal TUI that processing is starting
				}

				g, groupCtx := errgroup.WithContext(analysisCtx)
				if appCfg.MaxAgentConcurrency > 0 {
					g.SetLimit(appCfg.MaxAgentConcurrency)
				}

				fileResultsChan := make(chan string, len(filesToAnalyze))

				for i, filePath_ := range filesToAnalyze {
					filePath := filePath_ // Capture range variable
					fileNum := i + 1
					g.Go(func() error {
						// Check for context cancellation at the beginning of each file processing task
						select {
						case <-groupCtx.Done():
							log.Info("File analysis cancelled", "file", filePath, "reason", groupCtx.Err())
							return groupCtx.Err()
						default:
						}

						gLog := contextkeys.LoggerFromContext(groupCtx)
						if gLog == nil {
							gLog = log
							gLog.Warn("Logger not found in goroutine context, using main logger.", "file", filePath)
						}
						gLog.Info("Starting analysis for file", "file", filePath)
						fileContentBytes, err := os.ReadFile(filePath)
						if err != nil {
							msg := fmt.Sprintf("❌ Failed to read file %s: %v\n", filePath, err)
							fileResultsChan <- msg
							gLog.Error("Failed to read file", "file", filePath, "error", err)
							return nil
						}

						if tuiModel != nil {
							tuiModel.SetProgress(fileNum, len(filesToAnalyze), filepath.Base(filePath))
						} else {
							fmt.Printf("Analyzing %s (%d/%d)...\n", filePath, fileNum, len(filesToAnalyze))
						}
						agentProgressChan := make(chan orchestrator.AgentProgressUpdate, 10) // Buffer to be safe
						var progressWg sync.WaitGroup
						progressWg.Add(1)
						go func() {
							defer progressWg.Done()
							for update := range agentProgressChan {
								if tuiModel != nil {
									tuiModel.ProcessAgentUpdate(update)
								} else {
									progressMsg := fmt.Sprintf("  [%s] Agent: %s (%d/%d) - %s",
										filepath.Base(update.FilePath), update.AgentName, update.AgentIndex+1, update.TotalAgents, update.Status)
									if update.Status == orchestrator.StatusCompleted || update.Status == orchestrator.StatusFailed || update.Status == orchestrator.StatusTimedOut {
										progressMsg += fmt.Sprintf(" (%.2fs)", update.Duration.Seconds())
									}
									if update.Error != nil {
										if update.Status == orchestrator.StatusTimedOut {
											progressMsg += " - Timed out"
										} else {
											progressMsg += " - Error"
										}
									}
									fmt.Println(progressMsg)
								}
							}
						}()

						results, orchErr := agentOrchestrator.RunAgents(groupCtx, agentsToRun, filePath, string(fileContentBytes), agentProgressChan)
						progressWg.Wait()

						if orchErr != nil {
							msg := fmt.Sprintf("⚠️ Orchestrator error for %s: %v\n", filePath, orchErr)
							fileResultsChan <- msg
							gLog.Error("Orchestrator encountered an error for file", "file", filePath, "error", orchErr)
							if errors.Is(orchErr, context.Canceled) || errors.Is(orchErr, context.DeadlineExceeded) {
								return orchErr
							}
						}
						var output strings.Builder
						output.WriteString(fmt.Sprintf("Results for %s:\n", filePath))
						for _, result := range results {
							if result.Error != nil {
								var agentErr *agents.AgentError
								if errors.As(result.Error, &agentErr) {
									msg := fmt.Sprintf("⚠️ Error with %s (%s): %v\n", agentErr.AgentName, agentErr.Message, agentErr.Unwrap())
									output.WriteString(msg)
									gLog.Error("Agent execution failed", "agent_name", agentErr.AgentName, "file", result.File, "agent_message", agentErr.Message, "underlying_error", agentErr.Unwrap())
								} else if errors.Is(result.Error, ollama.ErrOllamaHostUnreachable) {
									msg := fmt.Sprintf(`⚠️ Error with %s: Could not connect to Ollama.

Please check:
1. Is Ollama running? Start it with:
   ollama serve

2. Is the Ollama URL correct? Current URL: %s
   Check your configuration or use --ollama-host-url flag.

3. Are there any firewall issues or network problems?
   Try: curl %s/api/tags to test connectivity.

`, result.AgentName, appCfg.OllamaHostURL, appCfg.OllamaHostURL)
									output.WriteString(msg)
									gLog.Error("Ollama host unreachable", "agent_name", result.AgentName, "file", result.File, "error", result.Error)
								} else if errors.Is(result.Error, ollama.ErrOllamaModelNotFound) {
									modelUsed := appCfg.DefaultModel
									if modelUsed == "" {
										gLog.Warn("DefaultModel in AppConfig is empty, which is unexpected.", "file", result.File)
										modelUsed = "[model_name_unavailable]"
									}
									msg := fmt.Sprintf(`⚠️ Error with %s: Model '%s' not found.

To fix this:
1. List available models:
   ollama list

2. Pull the model you want to use:
   ollama pull %s

3. Or use a different model:
   codex-lite analyze --default-model="llama2" [files...]

`, result.AgentName, modelUsed, modelUsed)
									output.WriteString(msg)
									gLog.Error("Ollama model not found", "agent_name", result.AgentName, "file", result.File, "error", result.Error, "model_used", modelUsed)
								} else {
									msg := fmt.Sprintf("⚠️ Error with %s: %v\n", result.AgentName, result.Error)
									output.WriteString(msg)
									gLog.Error("Generic error during agent analysis", "agent_name", result.AgentName, "file", result.File, "error", result.Error)
								}
							} else {
								output.WriteString(fmt.Sprintf("✅ %s analysis:\n", result.AgentName))
								if result.Output != "" {
									output.WriteString(fmt.Sprintf("   %s\n", strings.ReplaceAll(result.Output, "\n", "\n   ")))
								}
							}
						}
						output.WriteString("---\n")
						fileResultsChan <- output.String()
						return nil
					})
				}

				// Goroutine to collect file results and print/update TUI (for non-agent specific results)
				// This part is for the final string summary of each file.
				var allFileResultsWg sync.WaitGroup
				allFileResultsWg.Add(1)
				go func() {
					defer allFileResultsWg.Done()
					var allResults strings.Builder
					for result := range fileResultsChan { // Drains until closed
						if tuiModel != nil {
							allResults.WriteString(result)
							// Send content update to TUI program. A new message type might be better.
							// For now, using existing SetContent method, but this is called from a goroutine.
							// This needs to be a tea.Msg sent via tuiProgram.Send()
							tuiProgram.Send(tui.ContentUpdateMsg(allResults.String()))
						} else {
							fmt.Print(result)
						}
					}
				}()

				analysisErr := g.Wait()
				close(fileResultsChan)  // Close after g.Wait() ensures all producers are done
				allFileResultsWg.Wait() // Wait for the collector to finish processing all file results

				if tuiModel != nil {
					tuiModel.StopProcessing()
					if analysisErr != nil && !errors.Is(analysisErr, context.Canceled) && !errors.Is(analysisErr, context.DeadlineExceeded) {
						// Send error to TUI, it will display it and p.Quit() will be called by defer
						tuiProgram.Send(tui.ErrorMsg(analysisErr))
					}
				}

				if analysisErr != nil {
					log.Error("Error occurred during concurrent file analysis", "error", analysisErr)
					if errors.Is(analysisErr, context.Canceled) || errors.Is(analysisErr, context.DeadlineExceeded) {
						log.Info("Analysis was cancelled or timed out.")
						return nil // Suppress exit code for user-initiated cancellation / timeout
					}
					return fmt.Errorf("analysis group failed: %w", analysisErr)
				}

				log.Info("All file analyses finished.")
				if tuiModel == nil { // Only print if not in TUI mode (TUI has its own final view)
					fmt.Println("\nAll analyses finished.")
				}
				return nil
			}

			if noTui {
				// Run analysis logic directly in the main goroutine, using the main ctx
				return analyzeLogicFunc(ctx, nil, nil)
			} else {
				// TUI mode: Initialize TUI and run analysis logic in a separate goroutine
				tm := tui.NewModel("Ollama", appCfg.DefaultModel, time.Now().Format("20060102150405"))
				p := tea.NewProgram(&tm, tea.WithAltScreen(), tea.WithMouseCellMotion())
				tm.SetProgram(p)

				go analyzeLogicFunc(ctx, p, &tm) // Pass TUI program and model to the logic func

				// This will block until p.Quit() is called (either by analyzeLogicFunc or SIGINT)
				if _, err := p.Run(); err != nil {
					log.Error("TUI exited with error", "error", err)
					return err
				}
				// Check for an error stored on the model if p.Run() returns nil (e.g. from tea.Quit)
				if tm.Err() != nil { // Assuming an Err() getter on tui.Model
					log.Error("TUI model has an error after run", "error", tm.Err())
					return tm.Err()
				}
				return nil
			}
		},
	}
)

func init() {
	rootCmd.AddCommand(analyzeCmd)
	analyzeCmd.Flags().StringSliceVarP(&agentNames, "agent", "a", []string{}, "Comma-separated list of agent names to run (e.g., explain,syntax). Runs all if empty.")
	analyzeCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively search for files in directories.")
	analyzeCmd.Flags().IntVar(&maxDepth, "max-depth", -1, "Maximum directory depth to scan (-1 for unlimited).")
	analyzeCmd.Flags().StringSliceVar(&ignoreDirs, "ignore-dirs", []string{}, "Additional directories to ignore (comma-separated).")
	analyzeCmd.Flags().StringSliceVar(&extensions, "extensions", []string{}, "File extensions to analyze (comma-separated, without dots).")
	analyzeCmd.Flags().BoolVar(&noTui, "no-tui", false, "Disable the terminal user interface and use plain output.")
}
