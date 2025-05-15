package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/castrovroberto/codex-lite/internal/agents"
	"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/castrovroberto/codex-lite/internal/logger"
	"github.com/castrovroberto/codex-lite/internal/ollama" // Required for checking ollama specific errors
	"github.com/spf13/cobra"
)

var (
	agentsToRun []string
	outputFile  string
	modelToUse  string // Added model flag
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze [file_or_dir_paths...]",
	Short: "Analyze code files using selected AI agents",
	Long: `The analyze command processes one or more source code files or directories
using a specified set of AI agents. Each agent performs a specific type of analysis
(e.g., code explanation, smell detection, security audit).

Results from all agents are aggregated and can be printed to the console or
saved to a Markdown file.

Supported Agents:
  - explain: Explains what the code does.
  - smell: Identifies code smells and suggests improvements.
  - security: Audits code for potential security vulnerabilities.
  - syntax: Checks for syntax errors and potential issues.
  (Add more as they are implemented)

You can specify which agents to run using the --agents flag with a comma-separated list.
If --agents is not provided, a default set of agents might be used (currently TBD).
The AI model used by the agents can be specified with the --model flag.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.GetConfig()
		log := logger.Get()

		if modelToUse == "" {
			modelToUse = cfg.DefaultModel // Use default from config if not set by flag
		}
		if modelToUse == "" {
			return errors.New("no model specified via --model flag or in config as default_model")
		}

		log.Info("Starting analysis", "paths", args, "agents", agentsToRun, "model", modelToUse, "output_file", outputFile)

		// Initialize available agents
		// In a real app, this might be more dynamic (e.g., plugin system or registry)
		availableAgents := map[string]agents.Agent{
			"explain":  agents.NewExplainAgent(),
			"smell":    agents.NewSmellAgent(),
			"security": agents.NewSecurityAgent(),
			"syntax":   agents.NewSyntaxAgent(),
		}

		var selectedAgents []agents.Agent
		if len(agentsToRun) == 0 {
			// Default agents if none are specified (e.g., run all available)
			log.Info("No agents specified, running all available agents")
			for _, agent := range availableAgents {
				selectedAgents = append(selectedAgents, agent)
			}
		} else {
			for _, agentName := range agentsToRun {
				agent, ok := availableAgents[strings.ToLower(agentName)]
				if !ok {
					log.Warn("Unknown agent specified, skipping.", "agent_name", agentName)
					continue
				}
				selectedAgents = append(selectedAgents, agent)
			}
		}

		if len(selectedAgents) == 0 {
			return errors.New("no valid agents selected to run")
		}

		var filesToAnalyze []string
		for _, pathArg := range args {
			fileInfo, err := os.Stat(pathArg)
			if err != nil {
				log.Warn("Failed to stat path, skipping.", "path", pathArg, "error", err)
				continue
			}

			if fileInfo.IsDir() {
				log.Info("Scanning directory", "dir", pathArg)
				err := filepath.Walk(pathArg, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						log.Warn("Error accessing path during directory walk, skipping.", "path", path, "error", err)
						return nil // Continue walking
					}
					// TODO: Implement more sophisticated file type filtering (e.g., by extension, ignore .git, node_modules)
					if !info.IsDir() && strings.Contains(info.Name(), ".") { // Basic check for files
						log.Debug("Found file in directory", "file", path)
						filesToAnalyze = append(filesToAnalyze, path)
					}
					return nil
				})
				if err != nil {
					log.Error("Error walking directory", "dir", pathArg, "error", err)
				}
			} else {
				filesToAnalyze = append(filesToAnalyze, pathArg)
			}
		}

		if len(filesToAnalyze) == 0 {
			log.Info("No files found to analyze.")
			return nil
		}

		log.Info("Files to be analyzed", "count", len(filesToAnalyze), "files", filesToAnalyze)

		var allResults []agents.Result
		var resultsMutex sync.Mutex
		var wg sync.WaitGroup

		// Create a new context for this analysis run.
		// This allows for cancellation if needed, though not fully implemented here.
		analysisCtx, cancelAnalysis := context.WithCancel(cmd.Context())
		defer cancelAnalysis()

		for _, filePath := range filesToAnalyze {
			fileData, err := os.ReadFile(filePath)
			if err != nil {
				log.Error("Failed to read file, skipping.", "file", filePath, "error", err)
				fmt.Printf("⚠️ Error reading file %s: %v\n---\n", filePath, err)
				continue
			}

			fmt.Printf("📄 Analyzing file: %s (Model: %s, Ollama: %s)\n", filePath, modelToUse, cfg.OllamaHostURL)

			for _, agent := range selectedAgents {
				wg.Add(1)
				go func(ctx context.Context, agentInstance agents.Agent, currentFilePath string, currentFileData []byte) {
					defer wg.Done()
					log.Info("Running agent on file", "agent", agentInstance.Name(), "file", currentFilePath)
					fmt.Printf("🤖 Running %s...\n", agentInstance.Name())

					// Pass the analysisCtx which has the timeout from AppConfig (via agent's use of ollama.Query)
					result, agentErr := agentInstance.Analyze(ctx, modelToUse, currentFilePath, string(currentFileData))

					if agentErr != nil {
						var agtErr *agents.AgentError
						var ollamaHostErr, ollamaModelErr, ollamaInvalidRespErr, ollamaBadReqErr bool

						// Check for custom AgentError first
						if errors.As(agentErr, &agtErr) {
							log.Error("Agent execution failed",
								"agent_name", agtErr.AgentName,
								"file", currentFilePath,
								"agent_message", agtErr.Message,
								"underlying_error", agtErr.Unwrap(),
							)
							fmt.Printf("⚠️ Error with %s on %s (%s): %v\n", agtErr.AgentName, currentFilePath, agtErr.Message, agtErr.Unwrap())

							// Further check the unwrapped error for specific Ollama issues
							if agtErr.Unwrap() != nil {
								ollamaHostErr = errors.Is(agtErr.Unwrap(), ollama.ErrOllamaHostUnreachable)
								ollamaModelErr = errors.Is(agtErr.Unwrap(), ollama.ErrOllamaModelNotFound)
								ollamaInvalidRespErr = errors.Is(agtErr.Unwrap(), ollama.ErrOllamaInvalidResponse)
								ollamaBadReqErr = errors.Is(agtErr.Unwrap(), ollama.ErrOllamaBadRequest)
							}
						} else {
							// If not an AgentError, check for direct Ollama errors (less likely if agents wrap correctly)
							ollamaHostErr = errors.Is(agentErr, ollama.ErrOllamaHostUnreachable)
							ollamaModelErr = errors.Is(agentErr, ollama.ErrOllamaModelNotFound)
							ollamaInvalidRespErr = errors.Is(agentErr, ollama.ErrOllamaInvalidResponse)
							ollamaBadReqErr = errors.Is(agentErr, ollama.ErrOllamaBadRequest)

							// Log generic error if not an AgentError
							log.Error("Error during agent analysis", "agent_name", agentInstance.Name(), "file", currentFilePath, "error", agentErr)
							fmt.Printf("⚠️ Error with %s on %s: %v\n", agentInstance.Name(), currentFilePath, agentErr)
						}

						// Specific logging/messaging for Ollama errors if detected
						if ollamaHostErr {
							fmt.Printf("🔌   Detail: Could not connect to Ollama host.\n")
						}
						if ollamaModelErr {
							fmt.Printf("❓   Detail: Model '%s' not found by Ollama.\n", modelToUse)
						}
						if ollamaInvalidRespErr {
							fmt.Printf("⁉️   Detail: Invalid response from Ollama.\n")
						}
						if ollamaBadReqErr {
							fmt.Printf("🚫   Detail: Bad request sent to Ollama.\n")
						}

						fmt.Println("---") // Separator for agent errors
						return        // Do not add result if agent returned an error
					}

					log.Info("Agent finished successfully", "agent", agentInstance.Name(), "file", result.File)
					fmt.Printf("✅ %s analysis complete for %s.\n", result.AgentName, result.File)
					fmt.Printf("\n📘 [%s] - Result from %s:\n%s\n---\n", result.File, result.AgentName, result.Output)

					resultsMutex.Lock()
					allResults = append(allResults, result)
					resultsMutex.Unlock()

				}(analysisCtx, agent, filePath, fileData) // Pass copies to goroutine
			}
		}

		wg.Wait()
		log.Info("All agent analyses complete.")
		fmt.Println("\n🏁 All analyses finished.")

		if outputFile != "" {
			// TODO: Implement Markdown report generation (Task 14)
			log.Info("Output file specified, but report generation is not yet implemented.", "output_file", outputFile)
			fmt.Printf("\n📋 Report generation to '%s' is not yet implemented.\n", outputFile)
		} else {
			// If no output file, results are already printed to console during processing.
			if len(allResults) == 0 {
				fmt.Println("No analysis results were generated.")
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
	analyzeCmd.Flags().StringSliceVarP(&agentsToRun, "agents", "a", []string{}, "Comma-separated list of agent names to run (e.g., explain,smell)")
	analyzeCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output Markdown file to save the analysis report")
	analyzeCmd.Flags().StringVarP(&modelToUse, "model", "m", "", "AI model to use for analysis (overrides config default)")
}