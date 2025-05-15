package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"log/slog"
	"golang.org/x/sync/errgroup" // For concurrent file processing

	"github.com/castrovroberto/codex-lite/internal/agents"
	"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/castrovroberto/codex-lite/internal/contextkeys" // Import contextkeys
	"github.com/castrovroberto/codex-lite/internal/logger"
	"github.com/castrovroberto/codex-lite/internal/ollama"
	"github.com/castrovroberto/codex-lite/internal/orchestrator" // Import orchestrator
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
		Example: `codex-lite analyze --agent="explainer,syntax" --recursive ./...
codex-lite analyze path/to/your/file.go
codex-lite analyze "*.py"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			if loadCfgErr := config.LoadConfig(cfgFile); loadCfgErr != nil { // cfgFile from root.go
				// Use a basic logger if the main one hasn't been initialized due to config error
				slog.Error("Failed to load configuration on startup", "error", loadCfgErr)
				return fmt.Errorf("failed to load configuration: %w", loadCfgErr)
			}
			appCfg := config.Cfg // Use the global Cfg after successful load


			// Initialize logger based on loaded config
			logger.InitLogger(appCfg.LogLevel) // Corrected function name and removed LogFormat
			log := logger.Get() // Get the initialized logger
			log.Info("Codex Lite analyze starting...")
			log.Debug("Loaded configuration", "config", appCfg)

			// Create base context and inject config and logger
			baseCtx := cmd.Context() // Get context from Cobra commandS
			ctxWithValues := context.WithValue(baseCtx, contextkeys.ConfigKey, appCfg)
			ctxWithValues = context.WithValue(ctxWithValues, contextkeys.LoggerKey, log)

			// Initialize and register agents with the orchestrator
			agentOrchestrator := orchestrator.NewBasicOrchestrator()
			// Register all known agents
			if err := agentOrchestrator.RegisterAgent(agents.NewExplainAgent()); err != nil {
				log.Warn("Failed to register Explain agent", "error", err) // Warn, don't stop
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
			// Add other agents here for registration

			// Determine which agents to run
			var agentsToRun []string
			if len(agentNames) == 0 { // agentNames is the flag value
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
				return nil // Exit gracefully
			}

			log.Info("Selected agents to run", "agents", agentsToRun)
			fmt.Printf("Selected agents: %s\n\n", strings.Join(agentsToRun, ", "))

			// Determine files to analyze
			if len(args) == 0 {
				log.Error("No file patterns specified.")
				fmt.Println("Error: No file patterns specified.")
				cmd.Usage() // Show usage
				return errors.New("no file patterns specified")
			}

			var filesToAnalyze []string
			for _, pattern := range args {
				if recursive {
					// For recursive, we assume pattern is a directory or contains wildcards for directories
					// If pattern is a specific directory like "./src", filepath.WalkDir is good.
					// If pattern is like "./**/_.go", Glob might be better first, then walk matched dirs.
					// For simplicity, let's assume 'pattern' can be a starting directory for WalkDir.
					// A more robust solution might combine Glob for initial patterns and then WalkDir for directories.
					
					// Check if the pattern itself is a directory
					info, statErr := os.Stat(pattern)
					if statErr == nil && info.IsDir() {
						// It's a directory, walk it
						err := filepath.WalkDir(pattern, func(path string, d os.DirEntry, err error) error {
							if err != nil {
								log.Warn("Error accessing path during walk", "path", path, "error", err)
								return filepath.SkipDir // Or return err to stop walking this dir
							}
							if !d.IsDir() {
								filesToAnalyze = append(filesToAnalyze, path)
							}
							return nil
						})
						if err != nil {
							log.Error("Error walking directory", "directory", pattern, "error", err)
						}
					} else {
						// If not a directory, or stat failed, try Glob for recursive patterns like "**/_.go"
						// This part can be complex. For now, we'll stick to simple directory walking
						// or non-recursive Glob. A full ** glob might need a library or more complex logic.
						// For now, if recursive is true and pattern is not a dir, we'll Glob and then check if matches are dirs.
						matches, globErr := filepath.Glob(pattern)
						if globErr != nil {
							log.Error("Invalid file pattern for recursive glob", "pattern", pattern, "error", globErr)
							continue
						}
						for _, match := range matches {
							matchInfo, matchStatErr := os.Stat(match)
							if matchStatErr == nil {
								if matchInfo.IsDir() {
									// Walk this matched directory
									filepath.WalkDir(match, func(path string, d os.DirEntry, err error) error {
										if err != nil {
											log.Warn("Error accessing path during walk", "path", path, "error", err)
											return filepath.SkipDir
										}
										if !d.IsDir() {
											filesToAnalyze = append(filesToAnalyze, path)
										}
										return nil
									})
								} else {
									// It's a file from the glob match
									filesToAnalyze = append(filesToAnalyze, match)
								}
							}
						}
					}
				} else { // Not recursive
					matches, err := filepath.Glob(pattern)
					if err != nil {
						log.Error("Invalid file pattern", "pattern", pattern, "error", err)
						continue // Skip invalid patterns
					}
					for _, match := range matches {
						if info, err := os.Stat(match); err == nil && !info.IsDir() {
							filesToAnalyze = append(filesToAnalyze, match)
						}
					}
				}
			}

			if len(filesToAnalyze) == 0 {
				log.Info("No files found matching the pattern(s) or specified paths. Exiting.")
				fmt.Println("No files found to analyze.")
				return nil
			}

			// Deduplicate filesToAnalyze (in case of overlapping patterns or walks)
			seen := make(map[string]bool)
			uniqueFiles := []string{}
			for _, file := range filesToAnalyze {
				absPath, err := filepath.Abs(file)
				if err != nil {
					log.Warn("Could not get absolute path for file, using as is", "file", file, "error", err)
					absPath = file // Use original if Abs fails
				}
				if !seen[absPath] {
					seen[absPath] = true
					uniqueFiles = append(uniqueFiles, absPath)
				}
			}
			filesToAnalyze = uniqueFiles


			log.Info("Files to analyze", "count", len(filesToAnalyze))
			if len(filesToAnalyze) < 10 { // Log all files if list is short
				log.Debug("Target files", "files", filesToAnalyze)
			}


			// Use errgroup for concurrent file processing, passing the context with values
			g, analysisCtx := errgroup.WithContext(ctxWithValues)
			// g.SetLimit(appCfg.MaxConcurrentAnalyzers) // Example: Limit concurrency from config

			for _, filePath := range filesToAnalyze {
				filePath := filePath // Capture loop variable for goroutine
				g.Go(func() error {
					// Retrieve logger from context for this goroutine
					gLog := contextkeys.LoggerFromContext(analysisCtx)
					gLog.Info("Starting analysis for file", "file", filePath)

					fileContentBytes, err := os.ReadFile(filePath)
					if err != nil {
						gLog.Error("Failed to read file", "file", filePath, "error", err)
						fmt.Printf("❌ Failed to read file %s: %v\n", filePath, err)
						return nil // Return nil to allow other files to be processed
					}

					fmt.Printf("Analyzing %s...\n", filePath)

					// Use the orchestrator to run agents for this file
					results, orchErr := agentOrchestrator.RunAgents(analysisCtx, agentsToRun, filePath, string(fileContentBytes))
					if orchErr != nil {
						// This error is typically context cancellation from the orchestrator
						gLog.Error("Orchestrator encountered an error for file", "file", filePath, "error", orchErr)
						fmt.Printf("⚠️ Orchestrator error for %s: %v\n", filePath, orchErr)
						if errors.Is(orchErr, context.Canceled) || errors.Is(orchErr, context.DeadlineExceeded) {
							return orchErr // Propagate context errors to stop the errgroup
						}
						// For other orchestrator errors, decide if we stop or continue
						// return orchErr // Uncomment to stop all on any orchestrator error
					}

					// Process results collected by the orchestrator
					for _, result := range results {
						if result.Error != nil {
							// Handle individual agent error that the orchestrator collected
							var agentErr *agents.AgentError
							// No need to declare ollamaErrHostUnreachable, ollamaErrModelNotFound here
							// We will use ollama.ErrOllamaHostUnreachable directly

							if errors.As(result.Error, &agentErr) {
								gLog.Error("Agent execution failed", "agent_name", agentErr.AgentName, "file", result.File, "agent_message", agentErr.Message, "underlying_error", agentErr.Unwrap())
								fmt.Printf("⚠️ Error with %s (%s) on %s: %v\n", agentErr.AgentName, agentErr.Message, result.File, agentErr.Unwrap())
							} else if errors.Is(result.Error, ollama.ErrOllamaHostUnreachable) { // Compare with sentinel errors directly
								gLog.Error("Ollama host unreachable", "agent_name", result.AgentName, "file", result.File, "error", result.Error)
								fmt.Printf("⚠️ Error with %s on %s: Could not connect to Ollama: %v\n", result.AgentName, result.File, result.Error)
							} else if errors.Is(result.Error, ollama.ErrOllamaModelNotFound) {
								currentAppCfg := contextkeys.ConfigFromContext(analysisCtx) // Get current config for model name
								gLog.Error("Ollama model not found", "agent_name", result.AgentName, "file", result.File, "error", result.Error, "model_used", currentAppCfg.DefaultModel)
								fmt.Printf("⚠️ Error with %s on %s: The model '%s' was not found by Ollama: %v\n", result.AgentName, result.File, currentAppCfg.DefaultModel, result.Error)
							} else {
								gLog.Error("Generic error during agent analysis", "agent_name", result.AgentName, "file", result.File, "error", result.Error)
								fmt.Printf("⚠️ Error with %s on %s: %v\n", result.AgentName, result.File, result.Error)
							}
						} else {
							fmt.Printf("✅ %s analysis complete for %s.\n", result.AgentName, result.File)
							// You can access result.Output here if needed for further processing/display
							if result.Output != "" {
								// Indent output for clarity
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
					fmt.Println("---") // Separator for file results
					return nil         // Return nil to allow other files to be processed even if some agents failed
				})
			}

			// Wait for all file processing goroutines to complete.
			if err := g.Wait(); err != nil {
				log.Error("Error occurred during concurrent file analysis", "error", err)
				// This error is likely from a context cancellation or a returned error from a g.Go func
				// if we choose to propagate them.
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
	// cfgFile flag is added by rootCmd
}
