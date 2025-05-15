package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog" // Import slog for fallback logger
	"os"
	"path/filepath"
	"strings"

	// Added as per previous diff, anticipating Task 12
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/castrovroberto/codex-lite/internal/agents" // Needed for AppConfig type
	"github.com/castrovroberto/codex-lite/internal/contextkeys"
	"github.com/castrovroberto/codex-lite/internal/ollama"
	"github.com/castrovroberto/codex-lite/internal/orchestrator"
)

var (
	agentNames []string
	recursive  bool
	// cfgFile is defined in root.go

	analyzeCmd = &cobra.Command{
		Use:   "analyze [file patterns...]",
		Short: "Analyze code files using specified agents",
		Long: `Analyze command processes specified code files or patterns using a suite of agents.
You can specify which agents to run using the --agent flag (comma-separated).
If no agents are specified, all available agents will be run.
Use --recursive to scan directories.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Retrieve config and logger from the context
			appCfg := contextkeys.ConfigFromContext(cmd.Context()) // Returns config.AppConfig (struct)
			log := contextkeys.LoggerFromContext(cmd.Context())    // Returns *slog.Logger (pointer)

			// Check if logger was retrieved successfully (it can be nil)
			if log == nil {
				// This is a fallback. PersistentPreRunE should ensure a logger is always in context.
				fmt.Fprintln(os.Stderr, "Error: Logger not found in context. Using a temporary basic logger.")
				log = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
				// Depending on requirements, you might want to return an error here:
				// return errors.New("logger not found in context, cannot proceed")
			}

			// Check if config is meaningfully populated.
			// config.DefaultConfig() ensures OllamaHost (and DefaultModel) are non-empty.
			// If ConfigFromContext returned a zero struct (config.AppConfig{}), OllamaHost would be "".
			if appCfg.OllamaHostURL == "" {
				log.Error("Critical configuration (OllamaHost) is missing or empty. Check if PersistentPreRunE correctly loaded/set the configuration.")
				// Also print to stderr in case the logger itself is not fully functional
				fmt.Fprintln(os.Stderr, "Error: Critical configuration (OllamaHost) is missing or empty.")
				return errors.New("critical configuration (OllamaHost) missing or empty")
			}

			log.Info("Codex Lite analyze command starting...")
			log.Debug("Loaded configuration from context", "ollama_host", appCfg.OllamaHostURL, "default_model", appCfg.DefaultModel)

			ctxWithValues := cmd.Context() // The command's context already has the values

			// The rest of your RunE function...
			// Initialize and register agents with the orchestrator
			agentOrchestrator := orchestrator.NewBasicOrchestrator()
			// Register all known agents
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
			fmt.Printf("Selected agents: %s\n\n", strings.Join(agentsToRun, ", "))

			if len(args) == 0 {
				log.Error("No file patterns specified.")
				fmt.Println("Error: No file patterns specified.")
				cmd.Usage()
				return errors.New("no file patterns specified")
			}

			var filesToAnalyze []string
			for _, pattern := range args {
				if recursive {
					info, statErr := os.Stat(pattern)
					if statErr == nil && info.IsDir() {
						err := filepath.WalkDir(pattern, func(path string, d os.DirEntry, err error) error {
							if err != nil {
								// Log and continue if possible, or return err to stop
								log.Warn("Error accessing path, skipping", "path", path, "error", err)
								return nil // or return err if you want to stop walking on any error
							}

							if d.IsDir() {
								// Normalize directory name to lowercase for case-insensitive comparison
								dirName := strings.ToLower(d.Name())
								// Skip common VCS, build, and dependency directories
								if dirName == ".git" || dirName == ".svn" || dirName == ".hg" || dirName == ".bzr" ||
									dirName == "vendor" || dirName == "node_modules" ||
									dirName == "target" || dirName == "dist" || dirName == "build" {
									log.Debug("Skipping directory", "path", path, "reason", "standard skip list")
									return filepath.SkipDir
								}
								// If recursive is true (which it is to be in this block),
								// we don't need an explicit check for `!recursive && path != pattern` here,
								// as WalkDir handles the recursion.
							}
							if !d.IsDir() {
								// TODO: Implement file extension filtering here based on supported extensions
								// e.g., ext := utils.GetFileExtension(d.Name()); if isSupported(ext) { ... }
								filesToAnalyze = append(filesToAnalyze, path)
							}
							return nil
						})
						if err != nil {
							log.Error("Error walking directory", "directory", pattern, "error", err)
						}
					} else { // pattern is a file or glob
						matches, globErr := filepath.Glob(pattern)
						if globErr != nil {
							log.Error("Invalid file pattern for recursive glob", "pattern", pattern, "error", globErr)
							continue
						}
						for _, match := range matches {
							matchInfo, matchStatErr := os.Stat(match)
							if matchStatErr == nil {
								if matchInfo.IsDir() { // If a glob matches a directory and recursive is true
									filepath.WalkDir(match, func(path string, d os.DirEntry, err error) error {
										if err != nil {
											// Log and continue if possible, or return err to stop
											log.Warn("Error accessing path, skipping", "path", path, "error", err)
											return nil // or return err if you want to stop walking on any error
										}
										if d.IsDir() {
											// Normalize directory name to lowercase for case-insensitive comparison
											dirName := strings.ToLower(d.Name())
											// Skip common VCS, build, and dependency directories
											if dirName == ".git" || dirName == ".svn" || dirName == ".hg" || dirName == ".bzr" ||
												dirName == "vendor" || dirName == "node_modules" ||
												dirName == "target" || dirName == "dist" || dirName == "build" {
												log.Debug("Skipping directory", "path", path, "reason", "standard skip list")
												return filepath.SkipDir
											}
										}
										if !d.IsDir() {
											// TODO: Implement file extension filtering here
											filesToAnalyze = append(filesToAnalyze, path)
										}
										return nil
									})
								} else { // match is a file
									filesToAnalyze = append(filesToAnalyze, match)
								}
							}
						}
					}
				} else { // Not recursive
					matches, err := filepath.Glob(pattern)
					if err != nil {
						log.Error("Invalid file pattern", "pattern", pattern, "error", err)
						continue
					}
					for _, match := range matches {
						if info, err := os.Stat(match); err == nil && !info.IsDir() {
							// TODO: Implement file extension filtering here if needed for non-recursive single files
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
			if len(filesToAnalyze) < 10 {
				log.Debug("Target files", "files", filesToAnalyze)
			}

			g, analysisCtx := errgroup.WithContext(ctxWithValues)
			if appCfg.MaxConcurrentAnalyzers > 0 { // Set limit only if it's a positive value
				g.SetLimit(appCfg.MaxConcurrentAnalyzers)
			}

			for _, filePath := range filesToAnalyze {
				filePath := filePath
				g.Go(func() error {
					gLog := contextkeys.LoggerFromContext(analysisCtx)
					if gLog == nil { // Fallback for logger inside goroutine, though context should propagate it
						gLog = log // Use the main logger as a fallback
						gLog.Warn("Logger not found in goroutine context, using main logger.", "file", filePath)
					}
					gLog.Info("Starting analysis for file", "file", filePath)

					fileContentBytes, err := os.ReadFile(filePath)
					if err != nil {
						gLog.Error("Failed to read file", "file", filePath, "error", err)
						fmt.Printf("❌ Failed to read file %s: %v\n", filePath, err)
						return nil
					}

					fmt.Printf("Analyzing %s...\n", filePath)

					results, orchErr := agentOrchestrator.RunAgents(analysisCtx, agentsToRun, filePath, string(fileContentBytes))
					if orchErr != nil {
						gLog.Error("Orchestrator encountered an error for file", "file", filePath, "error", orchErr)
						fmt.Printf("⚠️ Orchestrator error for %s: %v\n", filePath, orchErr)
						if errors.Is(orchErr, context.Canceled) || errors.Is(orchErr, context.DeadlineExceeded) {
							return orchErr
						}
					}

					for _, result := range results {
						if result.Error != nil {
							var agentErr *agents.AgentError
							if errors.As(result.Error, &agentErr) {
								gLog.Error("Agent execution failed", "agent_name", agentErr.AgentName, "file", result.File, "agent_message", agentErr.Message, "underlying_error", agentErr.Unwrap())
								fmt.Printf("⚠️ Error with %s (%s) on %s: %v\n", agentErr.AgentName, agentErr.Message, result.File, agentErr.Unwrap())
							} else if errors.Is(result.Error, ollama.ErrOllamaHostUnreachable) {
								gLog.Error("Ollama host unreachable", "agent_name", result.AgentName, "file", result.File, "error", result.Error)
								fmt.Printf("⚠️ Error with %s on %s: Could not connect to Ollama: %v\n", result.AgentName, result.File, result.Error)
							} else if errors.Is(result.Error, ollama.ErrOllamaModelNotFound) {
								// Use appCfg captured from the RunE scope.
								modelUsed := appCfg.DefaultModel
								if modelUsed == "" {
									gLog.Warn("DefaultModel in AppConfig is empty, which is unexpected.", "file", result.File)
									modelUsed = "[model_name_unavailable]"
								}
								gLog.Error("Ollama model not found", "agent_name", result.AgentName, "file", result.File, "error", result.Error, "model_used", modelUsed)
								fmt.Printf("⚠️ Error with %s on %s: The model '%s' was not found by Ollama: %v\n", result.AgentName, result.File, modelUsed, result.Error)
							} else {
								gLog.Error("Generic error during agent analysis", "agent_name", result.AgentName, "file", result.File, "error", result.Error)
								fmt.Printf("⚠️ Error with %s on %s: %v\n", result.AgentName, result.File, result.Error)
							}
						} else {
							fmt.Printf("✅ %s analysis complete for %s.\n", result.AgentName, result.File)
							if result.Output != "" {
								outputLines := strings.Split(strings.TrimSpace(result.Output), "\n")
								for i, line := range outputLines {
									if i == 0 {
										fmt.Printf("   Output: %s\n", line)
									} else {
										fmt.Printf("           %s\n", line)
									}
								}
							}
						}
					}
					fmt.Println("---")
					return nil
				})
			}

			if err := g.Wait(); err != nil {
				log.Error("Error occurred during concurrent file analysis", "error", err)
				return fmt.Errorf("analysis group failed: %w", err)
			}

			log.Info("All file analyses finished.")
			fmt.Println("\nAll analyses finished.")
			return nil
		},
	}
)

func init() {
	rootCmd.AddCommand(analyzeCmd)
	analyzeCmd.Flags().StringSliceVarP(&agentNames, "agent", "a", []string{}, "Comma-separated list of agent names to run (e.g., explain,syntax). Runs all if empty.")
	analyzeCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively search for files in directories.")
}
