package vectorstore

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/castrovroberto/CGE/internal/textutils"
)

// Document represents a stored document with its embedding and metadata
type Document struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Embedding []float32              `json:"embedding"`
	Metadata  map[string]interface{} `json:"metadata"`
	Timestamp time.Time              `json:"timestamp"`
}

// SearchResult represents a search result with similarity score
type SearchResult struct {
	Document   *Document `json:"document"`
	Similarity float64   `json:"similarity"`
	Score      float64   `json:"score"` // Alias for similarity for backward compatibility
}

// VectorStore provides in-memory vector storage and similarity search
type VectorStore struct {
	documents map[string]*Document
	mutex     sync.RWMutex
	dimension int // Expected embedding dimension
}

// NewVectorStore creates a new in-memory vector store
func NewVectorStore(dimension int) *VectorStore {
	return &VectorStore{
		documents: make(map[string]*Document),
		dimension: dimension,
	}
}

// Add stores a document with its embedding in the vector store
func (vs *VectorStore) Add(id, content string, embedding []float32, metadata map[string]interface{}) error {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()

	if len(embedding) != vs.dimension && vs.dimension > 0 {
		return fmt.Errorf("embedding dimension mismatch: expected %d, got %d", vs.dimension, len(embedding))
	}

	// Set dimension on first document if not set
	if vs.dimension == 0 {
		vs.dimension = len(embedding)
	}

	// Normalize the embedding vector
	normalizedEmbedding := normalizeVector(embedding)

	doc := &Document{
		ID:        id,
		Content:   content,
		Embedding: normalizedEmbedding,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}

	vs.documents[id] = doc
	return nil
}

// AddChunk stores a text chunk with its embedding
func (vs *VectorStore) AddChunk(chunk textutils.TextChunk, embedding []float32) error {
	metadata := map[string]interface{}{
		"start_line":  chunk.StartLine,
		"end_line":    chunk.EndLine,
		"chunk_index": chunk.ChunkIndex,
	}

	// Add chunk metadata
	for k, v := range chunk.Metadata {
		metadata[k] = v
	}

	id := fmt.Sprintf("chunk_%d_%s", chunk.ChunkIndex, chunk.Metadata["file_path"])
	return vs.Add(id, chunk.Content, embedding, metadata)
}

// Search finds the most similar documents to the query embedding
func (vs *VectorStore) Search(queryEmbedding []float32, limit int) ([]*SearchResult, error) {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()

	if len(queryEmbedding) != vs.dimension {
		return nil, fmt.Errorf("query embedding dimension mismatch: expected %d, got %d", vs.dimension, len(queryEmbedding))
	}

	normalizedQuery := normalizeVector(queryEmbedding)
	var results []*SearchResult

	// Calculate similarity for all documents
	for _, doc := range vs.documents {
		similarity := cosineSimilarity(normalizedQuery, doc.Embedding)
		result := &SearchResult{
			Document:   doc,
			Similarity: similarity,
			Score:      similarity,
		}
		results = append(results, result)
	}

	// Sort by similarity (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	// Limit results
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// SearchByText searches for documents similar to the given text
// This requires an embedding function to convert text to embeddings
func (vs *VectorStore) SearchByText(text string, embedFunc func(string) ([]float32, error), limit int) ([]*SearchResult, error) {
	embedding, err := embedFunc(text)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding for query: %w", err)
	}

	return vs.Search(embedding, limit)
}

// Get retrieves a document by ID
func (vs *VectorStore) Get(id string) (*Document, bool) {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()

	doc, exists := vs.documents[id]
	return doc, exists
}

// Delete removes a document from the store
func (vs *VectorStore) Delete(id string) bool {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()

	_, exists := vs.documents[id]
	if exists {
		delete(vs.documents, id)
	}
	return exists
}

// List returns all document IDs
func (vs *VectorStore) List() []string {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()

	ids := make([]string, 0, len(vs.documents))
	for id := range vs.documents {
		ids = append(ids, id)
	}
	return ids
}

// Count returns the number of documents in the store
func (vs *VectorStore) Count() int {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()

	return len(vs.documents)
}

// Clear removes all documents from the store
func (vs *VectorStore) Clear() {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()

	vs.documents = make(map[string]*Document)
}

// FilterByMetadata searches documents that match the given metadata criteria
func (vs *VectorStore) FilterByMetadata(criteria map[string]interface{}) []*Document {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()

	var matches []*Document

	for _, doc := range vs.documents {
		if matchesMetadata(doc.Metadata, criteria) {
			matches = append(matches, doc)
		}
	}

	return matches
}

// SearchWithFilter combines similarity search with metadata filtering
func (vs *VectorStore) SearchWithFilter(queryEmbedding []float32, limit int, metadataFilter map[string]interface{}) ([]*SearchResult, error) {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()

	if len(queryEmbedding) != vs.dimension {
		return nil, fmt.Errorf("query embedding dimension mismatch: expected %d, got %d", vs.dimension, len(queryEmbedding))
	}

	normalizedQuery := normalizeVector(queryEmbedding)
	var results []*SearchResult

	// Calculate similarity for documents that match the filter
	for _, doc := range vs.documents {
		if metadataFilter != nil && !matchesMetadata(doc.Metadata, metadataFilter) {
			continue
		}

		similarity := cosineSimilarity(normalizedQuery, doc.Embedding)
		result := &SearchResult{
			Document:   doc,
			Similarity: similarity,
			Score:      similarity,
		}
		results = append(results, result)
	}

	// Sort by similarity (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	// Limit results
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// Export serializes the vector store to JSON
func (vs *VectorStore) Export() ([]byte, error) {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()

	data := struct {
		Documents map[string]*Document `json:"documents"`
		Dimension int                  `json:"dimension"`
	}{
		Documents: vs.documents,
		Dimension: vs.dimension,
	}

	return json.Marshal(data)
}

// Import loads a vector store from JSON data
func (vs *VectorStore) Import(data []byte) error {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()

	var importData struct {
		Documents map[string]*Document `json:"documents"`
		Dimension int                  `json:"dimension"`
	}

	if err := json.Unmarshal(data, &importData); err != nil {
		return fmt.Errorf("failed to unmarshal vector store data: %w", err)
	}

	vs.documents = importData.Documents
	vs.dimension = importData.Dimension

	return nil
}

// Helper functions

// normalizeVector normalizes a vector to unit length
func normalizeVector(vector []float32) []float32 {
	var magnitude float64
	for _, v := range vector {
		magnitude += float64(v * v)
	}
	magnitude = math.Sqrt(magnitude)

	if magnitude == 0 {
		return vector
	}

	normalized := make([]float32, len(vector))
	for i, v := range vector {
		normalized[i] = float32(float64(v) / magnitude)
	}

	return normalized
}

// cosineSimilarity calculates the cosine similarity between two normalized vectors
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct float64
	for i := range a {
		dotProduct += float64(a[i] * b[i])
	}

	// Since vectors are normalized, the cosine similarity is just the dot product
	return dotProduct
}

// matchesMetadata checks if document metadata matches the given criteria
func matchesMetadata(docMetadata, criteria map[string]interface{}) bool {
	for key, expectedValue := range criteria {
		docValue, exists := docMetadata[key]
		if !exists {
			return false
		}

		// Simple equality check - could be extended for more complex matching
		if docValue != expectedValue {
			return false
		}
	}
	return true
}
