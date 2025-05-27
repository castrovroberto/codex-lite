package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/castrovroberto/CGE/internal/textutils"
	"github.com/castrovroberto/CGE/internal/vectorstore"
)

// RetrieveContextTool implements intelligent context retrieval from the codebase
type RetrieveContextTool struct {
	workspaceRoot string
	vectorStore   *vectorstore.VectorStore
	llmClient     LLMClient
	modelName     string
	chunker       *textutils.Chunker
	summarizer    *textutils.Summarizer
}

// LLMClient interface for context retrieval (to avoid circular imports)
type LLMClient interface {
	Generate(ctx context.Context, modelName, prompt, systemPrompt string, tools []map[string]interface{}) (string, error)
	Embed(ctx context.Context, text string) ([]float32, error)
	SupportsEmbeddings() bool
}

// NewRetrieveContextTool creates a new context retrieval tool
func NewRetrieveContextTool(workspaceRoot string, llmClient LLMClient, modelName string) *RetrieveContextTool {
	// Initialize vector store with a reasonable dimension (will be set dynamically)
	vectorStore := vectorstore.NewVectorStore(0)

	// Configure chunker for context retrieval
	chunkOptions := textutils.ChunkOptions{
		Strategy:    textutils.ChunkBySemanticBoundaries,
		MaxSize:     150,
		OverlapSize: 15,
	}
	chunker := textutils.NewChunker(chunkOptions)

	// Configure summarizer
	summaryOptions := textutils.DefaultSummaryOptions()
	summaryOptions.MaxLength = 300
	summarizer := textutils.NewSummarizer(llmClient, modelName, summaryOptions)

	return &RetrieveContextTool{
		workspaceRoot: workspaceRoot,
		vectorStore:   vectorStore,
		llmClient:     llmClient,
		modelName:     modelName,
		chunker:       chunker,
		summarizer:    summarizer,
	}
}

func (t *RetrieveContextTool) Name() string {
	return "retrieve_context"
}

func (t *RetrieveContextTool) Description() string {
	return "Fetches relevant context from the codebase (specific files, or summaries of files) based on a natural language query"
}

func (t *RetrieveContextTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "Natural language query describing what context to retrieve"
			},
			"max_results": {
				"type": "integer",
				"description": "Maximum number of context pieces to return",
				"default": 5
			},
			"include_summaries": {
				"type": "boolean",
				"description": "Whether to include file summaries in addition to exact matches",
				"default": true
			},
			"file_filter": {
				"type": "string",
				"description": "Optional file path pattern to filter results (e.g., '*.go', 'internal/*')"
			}
		},
		"required": ["query"]
	}`)
}

type RetrieveContextParams struct {
	Query            string `json:"query"`
	MaxResults       int    `json:"max_results,omitempty"`
	IncludeSummaries bool   `json:"include_summaries,omitempty"`
	FileFilter       string `json:"file_filter,omitempty"`
}

func (t *RetrieveContextTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p RetrieveContextParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Set defaults
	if p.MaxResults == 0 {
		p.MaxResults = 5
	}

	// Try vector search first if embeddings are supported
	var contextResults []ContextResult
	var err error

	if t.llmClient.SupportsEmbeddings() && t.vectorStore.Count() > 0 {
		contextResults, err = t.vectorSearch(ctx, p)
		if err != nil {
			// Fall back to LLM-assisted search if vector search fails
			contextResults, err = t.llmAssistedSearch(ctx, p)
		}
	} else {
		// Use LLM-assisted search as fallback
		contextResults, err = t.llmAssistedSearch(ctx, p)
	}

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to retrieve context: %v", err),
		}, nil
	}

	// Format results
	formattedResults := t.formatResults(contextResults, p.IncludeSummaries)

	return &ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"query":         p.Query,
			"results_count": len(contextResults),
			"context":       formattedResults,
			"results":       contextResults,
		},
	}, nil
}

// ContextResult represents a piece of retrieved context
type ContextResult struct {
	FilePath  string  `json:"file_path"`
	Content   string  `json:"content"`
	StartLine int     `json:"start_line,omitempty"`
	EndLine   int     `json:"end_line,omitempty"`
	Relevance float64 `json:"relevance"`
	Type      string  `json:"type"` // "chunk", "file", "summary"
	Summary   string  `json:"summary,omitempty"`
}

// vectorSearch performs similarity search using embeddings
func (t *RetrieveContextTool) vectorSearch(ctx context.Context, params RetrieveContextParams) ([]ContextResult, error) {
	// Generate embedding for the query
	queryEmbedding, err := t.llmClient.Embed(ctx, params.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Prepare metadata filter if file filter is specified
	var metadataFilter map[string]interface{}
	if params.FileFilter != "" {
		// This is a simple implementation - could be enhanced with glob pattern matching
		metadataFilter = map[string]interface{}{
			"file_path": params.FileFilter,
		}
	}

	// Search vector store
	searchResults, err := t.vectorStore.SearchWithFilter(queryEmbedding, params.MaxResults*2, metadataFilter)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	// Convert to ContextResult
	var contextResults []ContextResult
	for _, result := range searchResults {
		if len(contextResults) >= params.MaxResults {
			break
		}

		contextResult := ContextResult{
			Content:   result.Document.Content,
			Relevance: result.Similarity,
			Type:      "chunk",
		}

		// Extract metadata
		if filePath, ok := result.Document.Metadata["file_path"].(string); ok {
			contextResult.FilePath = filePath
		}
		if startLine, ok := result.Document.Metadata["start_line"].(int); ok {
			contextResult.StartLine = startLine
		}
		if endLine, ok := result.Document.Metadata["end_line"].(int); ok {
			contextResult.EndLine = endLine
		}

		contextResults = append(contextResults, contextResult)
	}

	return contextResults, nil
}

// llmAssistedSearch uses the LLM to identify relevant files and content
func (t *RetrieveContextTool) llmAssistedSearch(ctx context.Context, params RetrieveContextParams) ([]ContextResult, error) {
	// First, get a list of files in the workspace
	fileList, err := t.getFileList()
	if err != nil {
		return nil, fmt.Errorf("failed to get file list: %w", err)
	}

	// Apply file filter if specified
	if params.FileFilter != "" {
		fileList = t.filterFiles(fileList, params.FileFilter)
	}

	// Ask LLM to identify relevant files
	relevantFiles, err := t.identifyRelevantFiles(ctx, params.Query, fileList)
	if err != nil {
		return nil, fmt.Errorf("failed to identify relevant files: %w", err)
	}

	// Read and process the relevant files
	var contextResults []ContextResult
	for i, filePath := range relevantFiles {
		if i >= params.MaxResults {
			break
		}

		content, err := readFileContent(filepath.Join(t.workspaceRoot, filePath))
		if err != nil {
			continue // Skip files that can't be read
		}

		// For large files, chunk and find most relevant sections
		if len(content) > 5000 {
			chunkResults, err := t.findRelevantChunks(ctx, content, filePath, params.Query)
			if err == nil && len(chunkResults) > 0 {
				contextResults = append(contextResults, chunkResults...)
				continue
			}
		}

		// For smaller files or if chunking fails, include the whole file
		contextResult := ContextResult{
			FilePath:  filePath,
			Content:   content,
			Relevance: 0.8, // Default relevance for LLM-identified files
			Type:      "file",
		}

		contextResults = append(contextResults, contextResult)
	}

	return contextResults, nil
}

// findRelevantChunks finds the most relevant chunks within a file
func (t *RetrieveContextTool) findRelevantChunks(ctx context.Context, content, filePath, query string) ([]ContextResult, error) {
	chunks, err := t.chunker.ChunkText(content)
	if err != nil {
		return nil, err
	}

	var contextResults []ContextResult

	// Score each chunk for relevance (simple keyword matching for now)
	for _, chunk := range chunks {
		relevance := t.calculateTextRelevance(chunk.Content, query)
		if relevance > 0.3 { // Threshold for relevance
			contextResult := ContextResult{
				FilePath:  filePath,
				Content:   chunk.Content,
				StartLine: chunk.StartLine,
				EndLine:   chunk.EndLine,
				Relevance: relevance,
				Type:      "chunk",
			}
			contextResults = append(contextResults, contextResult)
		}
	}

	// Sort by relevance and return top chunks
	if len(contextResults) > 3 {
		// Sort by relevance (descending)
		for i := 0; i < len(contextResults)-1; i++ {
			for j := i + 1; j < len(contextResults); j++ {
				if contextResults[i].Relevance < contextResults[j].Relevance {
					contextResults[i], contextResults[j] = contextResults[j], contextResults[i]
				}
			}
		}
		contextResults = contextResults[:3]
	}

	return contextResults, nil
}

// calculateTextRelevance calculates relevance score between text and query
func (t *RetrieveContextTool) calculateTextRelevance(text, query string) float64 {
	text = strings.ToLower(text)
	query = strings.ToLower(query)

	// Simple scoring based on keyword matches
	queryWords := strings.Fields(query)
	var score float64

	for _, word := range queryWords {
		if strings.Contains(text, word) {
			score += 1.0
		}
	}

	// Normalize by query length
	if len(queryWords) > 0 {
		score = score / float64(len(queryWords))
	}

	return score
}

// identifyRelevantFiles asks the LLM to identify relevant files
func (t *RetrieveContextTool) identifyRelevantFiles(ctx context.Context, query string, fileList []string) ([]string, error) {
	prompt := fmt.Sprintf(`Given the following query and list of files, identify the 5 most relevant files that would contain information related to the query.

Query: %s

Files:
%s

Please respond with only the file paths, one per line, in order of relevance.`, query, strings.Join(fileList, "\n"))

	systemPrompt := "You are an expert at analyzing codebases and identifying relevant files based on queries. Focus on file names, paths, and likely content."

	response, err := t.llmClient.Generate(ctx, t.modelName, prompt, systemPrompt, nil)
	if err != nil {
		return nil, err
	}

	// Parse the response to extract file paths
	lines := strings.Split(strings.TrimSpace(response), "\n")
	var relevantFiles []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && t.isValidFile(line, fileList) {
			relevantFiles = append(relevantFiles, line)
		}
	}

	return relevantFiles, nil
}

// formatResults formats the context results for display
func (t *RetrieveContextTool) formatResults(results []ContextResult, includeSummaries bool) string {
	var formatted strings.Builder

	formatted.WriteString(fmt.Sprintf("Found %d relevant context pieces:\n\n", len(results)))

	for i, result := range results {
		formatted.WriteString(fmt.Sprintf("## Result %d (Relevance: %.2f)\n", i+1, result.Relevance))
		formatted.WriteString(fmt.Sprintf("**File:** %s\n", result.FilePath))

		if result.StartLine > 0 {
			formatted.WriteString(fmt.Sprintf("**Lines:** %d-%d\n", result.StartLine, result.EndLine))
		}

		formatted.WriteString(fmt.Sprintf("**Type:** %s\n\n", result.Type))

		// Include content (truncated if too long)
		content := result.Content
		if len(content) > 1000 {
			content = content[:1000] + "...\n[Content truncated]"
		}

		formatted.WriteString("```\n")
		formatted.WriteString(content)
		formatted.WriteString("\n```\n\n")

		// Include summary if available and requested
		if includeSummaries && result.Summary != "" {
			formatted.WriteString(fmt.Sprintf("**Summary:** %s\n\n", result.Summary))
		}

		formatted.WriteString("---\n\n")
	}

	return formatted.String()
}

// Helper methods

func (t *RetrieveContextTool) getFileList() ([]string, error) {
	var files []string

	err := filepath.Walk(t.workspaceRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// Skip common directories
			if shouldSkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		// Only include source files
		if isSourceFile(path) {
			relPath, err := filepath.Rel(t.workspaceRoot, path)
			if err != nil {
				relPath = path
			}
			files = append(files, relPath)
		}

		return nil
	})

	return files, err
}

func (t *RetrieveContextTool) filterFiles(files []string, pattern string) []string {
	// Simple pattern matching - could be enhanced with proper glob support
	var filtered []string

	for _, file := range files {
		if strings.Contains(file, pattern) || matchesPattern(file, pattern) {
			filtered = append(filtered, file)
		}
	}

	return filtered
}

func (t *RetrieveContextTool) isValidFile(filePath string, fileList []string) bool {
	for _, file := range fileList {
		if file == filePath {
			return true
		}
	}
	return false
}

// IndexWorkspace indexes the workspace files for vector search
func (t *RetrieveContextTool) IndexWorkspace(ctx context.Context) error {
	if !t.llmClient.SupportsEmbeddings() {
		return fmt.Errorf("LLM client does not support embeddings")
	}

	files, err := t.getFileList()
	if err != nil {
		return fmt.Errorf("failed to get file list: %w", err)
	}

	for _, filePath := range files {
		fullPath := filepath.Join(t.workspaceRoot, filePath)
		content, err := readFileContent(fullPath)
		if err != nil {
			continue // Skip files that can't be read
		}

		// Skip very large files
		if len(content) > 100000 { // 100KB limit
			continue
		}

		// Chunk the file
		chunks, err := t.chunker.ChunkFile(fullPath)
		if err != nil {
			continue
		}

		// Generate embeddings and store chunks
		for _, chunk := range chunks {
			embedding, err := t.llmClient.Embed(ctx, chunk.Content)
			if err != nil {
				continue // Skip chunks that can't be embedded
			}

			err = t.vectorStore.AddChunk(chunk, embedding)
			if err != nil {
				continue // Skip chunks that can't be stored
			}
		}
	}

	return nil
}

// Helper functions (these would typically be in a shared utility package)

func shouldSkipDir(name string) bool {
	skipDirs := []string{".git", "node_modules", "vendor", ".vscode", ".idea", "target", "build", "dist"}
	for _, skip := range skipDirs {
		if name == skip {
			return true
		}
	}
	return false
}

func isSourceFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	sourceExts := []string{".go", ".py", ".js", ".ts", ".java", ".cpp", ".c", ".h", ".rs", ".rb", ".php", ".cs", ".md", ".txt", ".yaml", ".yml", ".json", ".toml"}

	for _, sourceExt := range sourceExts {
		if ext == sourceExt {
			return true
		}
	}
	return false
}

func matchesPattern(file, pattern string) bool {
	// Simple pattern matching - in a real implementation, use filepath.Match
	if strings.HasPrefix(pattern, "*.") {
		ext := pattern[1:]
		return strings.HasSuffix(file, ext)
	}
	if strings.HasSuffix(pattern, "/*") {
		dir := pattern[:len(pattern)-2]
		return strings.HasPrefix(file, dir+"/")
	}
	return false
}

func readFileContent(filepath string) (string, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
