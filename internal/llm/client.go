package llm

import "context"

// ThoughtResponse represents a structured response from deliberation
type ThoughtResponse struct {
	ThoughtContent  string   `json:"thought_content"`
	Confidence      float64  `json:"confidence"` // 0.0 to 1.0
	ReasoningSteps  []string `json:"reasoning_steps"`
	SuggestedAction string   `json:"suggested_action,omitempty"`
	Uncertainty     string   `json:"uncertainty,omitempty"` // Areas of uncertainty
}

// ConfidenceAssessment represents confidence in a decision
type ConfidenceAssessment struct {
	Score          float64                `json:"score"`          // 0.0 to 1.0
	Factors        map[string]float64     `json:"factors"`        // Contributing factors
	Uncertainties  []string               `json:"uncertainties"`  // Areas of uncertainty
	Recommendation string                 `json:"recommendation"` // Should proceed, retry, or abort
	Metadata       map[string]interface{} `json:"metadata"`
}

// Client defines the interface for interacting with a Large Language Model.
type Client interface {
	// Generate performs a non-streaming generation request.
	// modelName: The specific model to use (e.g., "llama3:latest").
	// prompt: The main user prompt.
	// systemPrompt: An optional system-level prompt to guide the LLM's behavior.
	// tools: An optional slice of maps, where each map describes a tool the LLM can use.
	//        The structure should be flexible to accommodate different provider schemas.
	Generate(ctx context.Context, modelName, prompt string, systemPrompt string, tools []map[string]interface{}) (string, error)

	// GenerateWithFunctions performs a generation request with function calling support.
	// Returns a structured response that can be either text or a function call.
	GenerateWithFunctions(ctx context.Context, modelName, prompt string, systemPrompt string, tools []ToolDefinition) (*FunctionCallResponse, error)

	// Stream performs a streaming generation request.
	// out: A channel to send generated text chunks to. The channel will be closed when generation is complete or an error occurs.
	// Other parameters are the same as Generate.
	Stream(ctx context.Context, modelName, prompt string, systemPrompt string, tools []map[string]interface{}, out chan<- string) error

	// ListAvailableModels retrieves a list of models available through the provider.
	ListAvailableModels(ctx context.Context) ([]string, error)

	// SupportsNativeFunctionCalling returns true if the provider supports native function calling
	SupportsNativeFunctionCalling() bool

	// Embed generates embeddings for the given text
	Embed(ctx context.Context, text string) ([]float32, error)

	// SupportsEmbeddings returns true if the provider supports text embeddings
	SupportsEmbeddings() bool

	// Extended methods for deliberation support

	// GenerateThought performs a deliberation step to generate internal reasoning
	// This is used for "thinking before acting" and should return structured thought content
	GenerateThought(ctx context.Context, modelName, prompt, context string) (*ThoughtResponse, error)

	// AssessConfidence evaluates confidence in a proposed action or decision
	// Returns a structured confidence assessment with contributing factors
	AssessConfidence(ctx context.Context, modelName, thought, proposedAction string) (*ConfidenceAssessment, error)

	// SupportsDeliberation returns true if the provider supports deliberation methods
	SupportsDeliberation() bool

	// TODO: Potentially add methods for token counting, specific model capabilities, etc.
}
