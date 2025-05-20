package llm

import "context"

// Client defines the interface for interacting with a Large Language Model.
type Client interface {
	// Generate performs a non-streaming generation request.
	// modelName: The specific model to use (e.g., "llama3:latest").
	// prompt: The main user prompt.
	// systemPrompt: An optional system-level prompt to guide the LLM's behavior.
	// tools: An optional slice of maps, where each map describes a tool the LLM can use.
	//        The structure should be flexible to accommodate different provider schemas.
	Generate(ctx context.Context, modelName, prompt string, systemPrompt string, tools []map[string]interface{}) (string, error)

	// Stream performs a streaming generation request.
	// out: A channel to send generated text chunks to. The channel will be closed when generation is complete or an error occurs.
	// Other parameters are the same as Generate.
	Stream(ctx context.Context, modelName, prompt string, systemPrompt string, tools []map[string]interface{}, out chan<- string) error

	// ListAvailableModels retrieves a list of models available through the provider.
	ListAvailableModels(ctx context.Context) ([]string, error)

	// TODO: Potentially add methods for token counting, specific model capabilities, etc.
}
