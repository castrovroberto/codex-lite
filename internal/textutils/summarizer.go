package textutils

import (
	"context"
	"fmt"
	"strings"
)

// LLMClient interface for summarization (to avoid circular imports)
type LLMClient interface {
	Generate(ctx context.Context, modelName, prompt, systemPrompt string, tools []map[string]interface{}) (string, error)
}

// SummaryOptions configures text summarization behavior
type SummaryOptions struct {
	MaxLength    int      // Maximum length of summary in characters
	Style        string   // Summary style: "brief", "detailed", "technical"
	PreserveTags []string // Code elements to preserve (functions, classes, etc.)
	IncludeCode  bool     // Whether to include code snippets in summary
}

// DefaultSummaryOptions returns sensible defaults for summarization
func DefaultSummaryOptions() SummaryOptions {
	return SummaryOptions{
		MaxLength:    500,
		Style:        "technical",
		PreserveTags: []string{"func", "class", "interface", "type"},
		IncludeCode:  true,
	}
}

// Summarizer handles text summarization operations
type Summarizer struct {
	llmClient LLMClient
	modelName string
	options   SummaryOptions
}

// NewSummarizer creates a new text summarizer
func NewSummarizer(llmClient LLMClient, modelName string, options SummaryOptions) *Summarizer {
	return &Summarizer{
		llmClient: llmClient,
		modelName: modelName,
		options:   options,
	}
}

// SummarizeText creates a summary of the given text
func (s *Summarizer) SummarizeText(ctx context.Context, text string) (string, error) {
	if len(text) == 0 {
		return "", fmt.Errorf("cannot summarize empty text")
	}

	// If text is already short enough, return as-is
	if len(text) <= s.options.MaxLength {
		return text, nil
	}

	prompt := s.buildSummarizationPrompt(text)
	systemPrompt := s.buildSystemPrompt()

	summary, err := s.llmClient.Generate(ctx, s.modelName, prompt, systemPrompt, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate summary: %w", err)
	}

	return strings.TrimSpace(summary), nil
}

// SummarizeChunks creates summaries for multiple text chunks
func (s *Summarizer) SummarizeChunks(ctx context.Context, chunks []TextChunk) ([]string, error) {
	summaries := make([]string, len(chunks))

	for i, chunk := range chunks {
		summary, err := s.SummarizeText(ctx, chunk.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to summarize chunk %d: %w", i, err)
		}
		summaries[i] = summary
	}

	return summaries, nil
}

// SummarizeFile reads and summarizes a file
func (s *Summarizer) SummarizeFile(ctx context.Context, filepath string) (string, error) {
	content, err := readFileContent(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filepath, err)
	}

	// For large files, chunk first then summarize
	if len(content) > 10000 { // 10KB threshold
		return s.summarizeLargeFile(ctx, content, filepath)
	}

	summary, err := s.SummarizeText(ctx, content)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("File: %s\n%s", filepath, summary), nil
}

// summarizeLargeFile handles summarization of large files using chunking
func (s *Summarizer) summarizeLargeFile(ctx context.Context, content, filepath string) (string, error) {
	// Create chunker with appropriate settings for summarization
	chunkOptions := ChunkOptions{
		Strategy:    ChunkBySemanticBoundaries,
		MaxSize:     200, // Smaller chunks for better summarization
		OverlapSize: 20,
	}
	chunker := NewChunker(chunkOptions)

	chunks, err := chunker.ChunkText(content)
	if err != nil {
		return "", fmt.Errorf("failed to chunk file content: %w", err)
	}

	// Summarize each chunk
	chunkSummaries, err := s.SummarizeChunks(ctx, chunks)
	if err != nil {
		return "", err
	}

	// Combine chunk summaries into a final summary
	combinedSummary := strings.Join(chunkSummaries, "\n\n")

	// If the combined summary is still too long, summarize it again
	if len(combinedSummary) > s.options.MaxLength*2 {
		finalSummary, err := s.SummarizeText(ctx, combinedSummary)
		if err != nil {
			return "", fmt.Errorf("failed to create final summary: %w", err)
		}
		return fmt.Sprintf("File: %s\n%s", filepath, finalSummary), nil
	}

	return fmt.Sprintf("File: %s\n%s", filepath, combinedSummary), nil
}

// buildSummarizationPrompt creates the prompt for summarization
func (s *Summarizer) buildSummarizationPrompt(text string) string {
	var prompt strings.Builder

	prompt.WriteString("Please summarize the following text")

	switch s.options.Style {
	case "brief":
		prompt.WriteString(" in a brief, concise manner")
	case "detailed":
		prompt.WriteString(" in detail, preserving important information")
	case "technical":
		prompt.WriteString(" focusing on technical aspects and key functionality")
	}

	if s.options.IncludeCode && containsCode(text) {
		prompt.WriteString(". Include important code structures and function signatures")
	}

	if len(s.options.PreserveTags) > 0 {
		prompt.WriteString(fmt.Sprintf(". Pay special attention to: %s", strings.Join(s.options.PreserveTags, ", ")))
	}

	prompt.WriteString(fmt.Sprintf(". Keep the summary under %d characters.\n\n", s.options.MaxLength))
	prompt.WriteString("Text to summarize:\n")
	prompt.WriteString(text)

	return prompt.String()
}

// buildSystemPrompt creates the system prompt for summarization
func (s *Summarizer) buildSystemPrompt() string {
	return `You are an expert at creating concise, informative summaries of code and technical documentation. 
Focus on the main purpose, key functionality, and important details. 
Preserve technical accuracy while making the content accessible.
If the text contains code, highlight the main functions, classes, and their purposes.`
}

// SlidingWindowSummary creates a summary using a sliding window approach
func (s *Summarizer) SlidingWindowSummary(ctx context.Context, text string, windowSize int) (string, error) {
	if len(text) <= windowSize {
		return s.SummarizeText(ctx, text)
	}

	var summaries []string
	lines := strings.Split(text, "\n")

	for i := 0; i < len(lines); i += windowSize / 2 { // 50% overlap
		end := i + windowSize
		if end > len(lines) {
			end = len(lines)
		}

		window := strings.Join(lines[i:end], "\n")
		summary, err := s.SummarizeText(ctx, window)
		if err != nil {
			return "", fmt.Errorf("failed to summarize window starting at line %d: %w", i, err)
		}

		summaries = append(summaries, summary)

		if end >= len(lines) {
			break
		}
	}

	// Combine all window summaries
	combinedSummary := strings.Join(summaries, "\n\n")

	// Final summarization pass if needed
	if len(combinedSummary) > s.options.MaxLength*2 {
		return s.SummarizeText(ctx, combinedSummary)
	}

	return combinedSummary, nil
}

// Helper functions

// containsCode checks if text appears to contain code
func containsCode(text string) bool {
	codeIndicators := []string{
		"func ", "function ", "class ", "def ", "interface ", "type ",
		"import ", "package ", "module ", "const ", "var ",
		"{", "}", "(", ")", "[", "]", "=", "==", "!=",
	}

	lowerText := strings.ToLower(text)
	codeCount := 0

	for _, indicator := range codeIndicators {
		if strings.Contains(lowerText, indicator) {
			codeCount++
		}
	}

	// If we find multiple code indicators, it's likely code
	return codeCount >= 3
}
