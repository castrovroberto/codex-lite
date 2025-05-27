package context

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/castrovroberto/CGE/internal/textutils"
	"github.com/castrovroberto/CGE/internal/vectorstore"
)

// LLMClient interface for context management (to avoid circular imports)
type LLMClient interface {
	Generate(ctx context.Context, modelName, prompt, systemPrompt string, tools []map[string]interface{}) (string, error)
	Embed(ctx context.Context, text string) ([]float32, error)
	SupportsEmbeddings() bool
}

// ContextManager provides centralized context management and retrieval
type ContextManager struct {
	workspaceRoot string
	gatherer      *Gatherer
	vectorStore   *vectorstore.VectorStore
	llmClient     LLMClient
	modelName     string
	chunker       *textutils.Chunker
	summarizer    *textutils.Summarizer

	// Cache management
	cache        map[string]*CachedContext
	cacheMutex   sync.RWMutex
	maxCacheSize int
	cacheTimeout time.Duration

	// Indexing state
	indexed       bool
	indexMutex    sync.RWMutex
	lastIndexTime time.Time
}

// CachedContext represents cached context information
type CachedContext struct {
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Query     string    `json:"query"`
	Results   int       `json:"results"`
}

// ContextOptions configures context management behavior
type ContextOptions struct {
	MaxCacheSize     int           `json:"max_cache_size"`
	CacheTimeout     time.Duration `json:"cache_timeout"`
	ChunkSize        int           `json:"chunk_size"`
	ChunkOverlap     int           `json:"chunk_overlap"`
	SummaryMaxLength int           `json:"summary_max_length"`
	VectorDimension  int           `json:"vector_dimension"`
}

// DefaultContextOptions returns sensible defaults for context management
func DefaultContextOptions() ContextOptions {
	return ContextOptions{
		MaxCacheSize:     100,
		CacheTimeout:     30 * time.Minute,
		ChunkSize:        150,
		ChunkOverlap:     15,
		SummaryMaxLength: 500,
		VectorDimension:  0, // Will be set dynamically
	}
}

// NewContextManager creates a new context manager
func NewContextManager(workspaceRoot string, llmClient LLMClient, modelName string, options ContextOptions) *ContextManager {
	gatherer := NewGatherer(workspaceRoot)
	vectorStore := vectorstore.NewVectorStore(options.VectorDimension)

	// Configure chunker
	chunkOptions := textutils.ChunkOptions{
		Strategy:    textutils.ChunkBySemanticBoundaries,
		MaxSize:     options.ChunkSize,
		OverlapSize: options.ChunkOverlap,
	}
	chunker := textutils.NewChunker(chunkOptions)

	// Configure summarizer
	summaryOptions := textutils.DefaultSummaryOptions()
	summaryOptions.MaxLength = options.SummaryMaxLength
	summarizer := textutils.NewSummarizer(llmClient, modelName, summaryOptions)

	return &ContextManager{
		workspaceRoot: workspaceRoot,
		gatherer:      gatherer,
		vectorStore:   vectorStore,
		llmClient:     llmClient,
		modelName:     modelName,
		chunker:       chunker,
		summarizer:    summarizer,
		cache:         make(map[string]*CachedContext),
		maxCacheSize:  options.MaxCacheSize,
		cacheTimeout:  options.CacheTimeout,
	}
}

// GetBasicContext retrieves basic codebase context information
func (cm *ContextManager) GetBasicContext() (*ContextInfo, error) {
	return cm.gatherer.GatherContext()
}

// RetrieveContext retrieves relevant context based on a query
func (cm *ContextManager) RetrieveContext(ctx context.Context, query string, maxResults int) (*ContextResponse, error) {
	// Check cache first
	if cached := cm.getCachedContext(query); cached != nil {
		return &ContextResponse{
			Query:     query,
			Content:   cached.Content,
			Cached:    true,
			Timestamp: cached.Timestamp,
		}, nil
	}

	// Ensure workspace is indexed if embeddings are supported
	if cm.llmClient.SupportsEmbeddings() {
		if err := cm.ensureIndexed(ctx); err != nil {
			return nil, fmt.Errorf("failed to ensure workspace is indexed: %w", err)
		}
	}

	var contextPieces []ContextPiece
	var err error

	// Try vector search first if available
	if cm.llmClient.SupportsEmbeddings() && cm.vectorStore.Count() > 0 {
		contextPieces, err = cm.vectorSearch(ctx, query, maxResults)
		if err != nil {
			// Fall back to LLM-assisted search
			contextPieces, err = cm.llmAssistedSearch(ctx, query, maxResults)
		}
	} else {
		// Use LLM-assisted search
		contextPieces, err = cm.llmAssistedSearch(ctx, query, maxResults)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve context: %w", err)
	}

	// Format and cache the results
	content := cm.formatContextPieces(contextPieces)
	cm.cacheContext(query, content, len(contextPieces))

	return &ContextResponse{
		Query:     query,
		Content:   content,
		Pieces:    contextPieces,
		Cached:    false,
		Timestamp: time.Now(),
	}, nil
}

// ContextResponse represents the response from context retrieval
type ContextResponse struct {
	Query     string         `json:"query"`
	Content   string         `json:"content"`
	Pieces    []ContextPiece `json:"pieces,omitempty"`
	Cached    bool           `json:"cached"`
	Timestamp time.Time      `json:"timestamp"`
}

// ContextPiece represents a piece of retrieved context
type ContextPiece struct {
	FilePath  string                 `json:"file_path"`
	Content   string                 `json:"content"`
	StartLine int                    `json:"start_line,omitempty"`
	EndLine   int                    `json:"end_line,omitempty"`
	Relevance float64                `json:"relevance"`
	Type      string                 `json:"type"` // "chunk", "file", "summary"
	Summary   string                 `json:"summary,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// vectorSearch performs similarity search using embeddings
func (cm *ContextManager) vectorSearch(ctx context.Context, query string, maxResults int) ([]ContextPiece, error) {
	// Generate embedding for the query
	queryEmbedding, err := cm.llmClient.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Search vector store
	searchResults, err := cm.vectorStore.Search(queryEmbedding, maxResults*2)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	// Convert to ContextPiece
	var contextPieces []ContextPiece
	for _, result := range searchResults {
		if len(contextPieces) >= maxResults {
			break
		}

		piece := ContextPiece{
			Content:   result.Document.Content,
			Relevance: result.Similarity,
			Type:      "chunk",
			Metadata:  result.Document.Metadata,
		}

		// Extract metadata
		if filePath, ok := result.Document.Metadata["file_path"].(string); ok {
			piece.FilePath = filePath
		}
		if startLine, ok := result.Document.Metadata["start_line"].(int); ok {
			piece.StartLine = startLine
		}
		if endLine, ok := result.Document.Metadata["end_line"].(int); ok {
			piece.EndLine = endLine
		}

		contextPieces = append(contextPieces, piece)
	}

	return contextPieces, nil
}

// llmAssistedSearch uses the LLM to identify and retrieve relevant context
func (cm *ContextManager) llmAssistedSearch(ctx context.Context, query string, maxResults int) ([]ContextPiece, error) {
	// Get basic context first
	basicContext, err := cm.GetBasicContext()
	if err != nil {
		return nil, fmt.Errorf("failed to get basic context: %w", err)
	}

	// Use LLM to identify relevant files
	prompt := fmt.Sprintf(`Based on the following query and codebase information, identify the most relevant files and content areas that would help answer the query.

Query: %s

Codebase Structure:
%s

Please respond with a list of file paths and brief explanations of why they're relevant, one per line in the format:
filepath: explanation`, query, basicContext.FileStructure)

	systemPrompt := "You are an expert at analyzing codebases and identifying relevant files and content areas based on queries."

	response, err := cm.llmClient.Generate(ctx, cm.modelName, prompt, systemPrompt, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get LLM assistance: %w", err)
	}

	// Parse the response and read relevant files
	return cm.parseAndReadFiles(response, maxResults)
}

// parseAndReadFiles parses LLM response and reads the identified files
func (cm *ContextManager) parseAndReadFiles(response string, maxResults int) ([]ContextPiece, error) {
	lines := strings.Split(strings.TrimSpace(response), "\n")
	var contextPieces []ContextPiece

	for i, line := range lines {
		if i >= maxResults {
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse "filepath: explanation" format
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 1 {
			continue
		}

		filePath := strings.TrimSpace(parts[0])
		explanation := ""
		if len(parts) > 1 {
			explanation = strings.TrimSpace(parts[1])
		}

		// Read file content
		fullPath := filepath.Join(cm.workspaceRoot, filePath)
		content, err := readFileContent(fullPath)
		if err != nil {
			continue // Skip files that can't be read
		}

		// For large files, try to find most relevant sections
		if len(content) > 5000 {
			chunks, err := cm.chunker.ChunkText(content)
			if err == nil && len(chunks) > 0 {
				// Find most relevant chunk (simple keyword matching)
				bestChunk := chunks[0]
				bestScore := 0.0

				for _, chunk := range chunks {
					score := cm.calculateRelevance(chunk.Content, explanation)
					if score > bestScore {
						bestScore = score
						bestChunk = chunk
					}
				}

				piece := ContextPiece{
					FilePath:  filePath,
					Content:   bestChunk.Content,
					StartLine: bestChunk.StartLine,
					EndLine:   bestChunk.EndLine,
					Relevance: bestScore,
					Type:      "chunk",
					Summary:   explanation,
				}
				contextPieces = append(contextPieces, piece)
				continue
			}
		}

		// For smaller files, include the whole file
		piece := ContextPiece{
			FilePath:  filePath,
			Content:   content,
			Relevance: 0.8, // Default relevance for LLM-identified files
			Type:      "file",
			Summary:   explanation,
		}
		contextPieces = append(contextPieces, piece)
	}

	return contextPieces, nil
}

// calculateRelevance calculates relevance score between content and explanation
func (cm *ContextManager) calculateRelevance(content, explanation string) float64 {
	if explanation == "" {
		return 0.5
	}

	content = strings.ToLower(content)
	explanation = strings.ToLower(explanation)

	words := strings.Fields(explanation)
	matches := 0

	for _, word := range words {
		if strings.Contains(content, word) {
			matches++
		}
	}

	if len(words) == 0 {
		return 0.5
	}

	return float64(matches) / float64(len(words))
}

// formatContextPieces formats context pieces into a readable string
func (cm *ContextManager) formatContextPieces(pieces []ContextPiece) string {
	var formatted strings.Builder

	formatted.WriteString(fmt.Sprintf("Retrieved %d relevant context pieces:\n\n", len(pieces)))

	for i, piece := range pieces {
		formatted.WriteString(fmt.Sprintf("## Context %d (Relevance: %.2f)\n", i+1, piece.Relevance))
		formatted.WriteString(fmt.Sprintf("**File:** %s\n", piece.FilePath))

		if piece.StartLine > 0 {
			formatted.WriteString(fmt.Sprintf("**Lines:** %d-%d\n", piece.StartLine, piece.EndLine))
		}

		formatted.WriteString(fmt.Sprintf("**Type:** %s\n", piece.Type))

		if piece.Summary != "" {
			formatted.WriteString(fmt.Sprintf("**Summary:** %s\n", piece.Summary))
		}

		formatted.WriteString("\n```\n")

		// Truncate very long content
		content := piece.Content
		if len(content) > 1500 {
			content = content[:1500] + "\n... [Content truncated]"
		}

		formatted.WriteString(content)
		formatted.WriteString("\n```\n\n")

		if i < len(pieces)-1 {
			formatted.WriteString("---\n\n")
		}
	}

	return formatted.String()
}

// IndexWorkspace indexes the workspace for vector search
func (cm *ContextManager) IndexWorkspace(ctx context.Context) error {
	cm.indexMutex.Lock()
	defer cm.indexMutex.Unlock()

	if !cm.llmClient.SupportsEmbeddings() {
		return fmt.Errorf("LLM client does not support embeddings")
	}

	// Clear existing index
	cm.vectorStore.Clear()

	// Get all source files
	files, err := cm.getSourceFiles()
	if err != nil {
		return fmt.Errorf("failed to get source files: %w", err)
	}

	// Index each file
	for _, filePath := range files {
		if err := cm.indexFile(ctx, filePath); err != nil {
			// Log error but continue with other files
			continue
		}
	}

	cm.indexed = true
	cm.lastIndexTime = time.Now()

	return nil
}

// indexFile indexes a single file
func (cm *ContextManager) indexFile(ctx context.Context, filePath string) error {
	content, err := readFileContent(filePath)
	if err != nil {
		return err
	}

	// Skip very large files
	if len(content) > 100000 { // 100KB limit
		return nil
	}

	// Chunk the file
	chunks, err := cm.chunker.ChunkText(content)
	if err != nil {
		return err
	}

	// Add file metadata to chunks
	relPath, _ := filepath.Rel(cm.workspaceRoot, filePath)
	for i := range chunks {
		if chunks[i].Metadata == nil {
			chunks[i].Metadata = make(map[string]string)
		}
		chunks[i].Metadata["file_path"] = relPath
	}

	// Generate embeddings and store chunks
	for _, chunk := range chunks {
		embedding, err := cm.llmClient.Embed(ctx, chunk.Content)
		if err != nil {
			continue // Skip chunks that can't be embedded
		}

		err = cm.vectorStore.AddChunk(chunk, embedding)
		if err != nil {
			continue // Skip chunks that can't be stored
		}
	}

	return nil
}

// ensureIndexed ensures the workspace is indexed
func (cm *ContextManager) ensureIndexed(ctx context.Context) error {
	cm.indexMutex.RLock()
	needsIndexing := !cm.indexed || time.Since(cm.lastIndexTime) > 24*time.Hour
	cm.indexMutex.RUnlock()

	if needsIndexing {
		return cm.IndexWorkspace(ctx)
	}

	return nil
}

// Cache management methods

func (cm *ContextManager) getCachedContext(query string) *CachedContext {
	cm.cacheMutex.RLock()
	defer cm.cacheMutex.RUnlock()

	cached, exists := cm.cache[query]
	if !exists {
		return nil
	}

	// Check if cache entry is still valid
	if time.Since(cached.Timestamp) > cm.cacheTimeout {
		delete(cm.cache, query)
		return nil
	}

	return cached
}

func (cm *ContextManager) cacheContext(query, content string, resultCount int) {
	cm.cacheMutex.Lock()
	defer cm.cacheMutex.Unlock()

	// Remove oldest entries if cache is full
	if len(cm.cache) >= cm.maxCacheSize {
		cm.evictOldestCacheEntry()
	}

	cm.cache[query] = &CachedContext{
		Content:   content,
		Timestamp: time.Now(),
		Query:     query,
		Results:   resultCount,
	}
}

func (cm *ContextManager) evictOldestCacheEntry() {
	var oldestKey string
	var oldestTime time.Time

	for key, cached := range cm.cache {
		if oldestKey == "" || cached.Timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = cached.Timestamp
		}
	}

	if oldestKey != "" {
		delete(cm.cache, oldestKey)
	}
}

// ClearCache clears the context cache
func (cm *ContextManager) ClearCache() {
	cm.cacheMutex.Lock()
	defer cm.cacheMutex.Unlock()
	cm.cache = make(map[string]*CachedContext)
}

// GetStats returns context manager statistics
func (cm *ContextManager) GetStats() map[string]interface{} {
	cm.cacheMutex.RLock()
	cm.indexMutex.RLock()
	defer cm.cacheMutex.RUnlock()
	defer cm.indexMutex.RUnlock()

	return map[string]interface{}{
		"cache_size":        len(cm.cache),
		"max_cache_size":    cm.maxCacheSize,
		"indexed":           cm.indexed,
		"last_index_time":   cm.lastIndexTime,
		"vector_store_size": cm.vectorStore.Count(),
	}
}

// Helper functions

func (cm *ContextManager) getSourceFiles() ([]string, error) {
	var files []string

	err := filepath.Walk(cm.workspaceRoot, func(path string, info os.FileInfo, err error) error {
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
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

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

func readFileContent(filepath string) (string, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
