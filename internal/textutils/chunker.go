package textutils

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ChunkStrategy defines how text should be chunked
type ChunkStrategy int

const (
	ChunkByLines ChunkStrategy = iota
	ChunkByTokens
	ChunkBySemanticBoundaries
)

// ChunkOptions configures text chunking behavior
type ChunkOptions struct {
	Strategy     ChunkStrategy
	MaxSize      int     // Maximum chunk size (lines or approximate tokens)
	OverlapSize  int     // Number of lines/tokens to overlap between chunks
	OverlapRatio float64 // Alternative to OverlapSize: ratio of chunk size (0.0-1.0)
}

// DefaultChunkOptions returns sensible defaults for text chunking
func DefaultChunkOptions() ChunkOptions {
	return ChunkOptions{
		Strategy:     ChunkByLines,
		MaxSize:      100,
		OverlapSize:  10,
		OverlapRatio: 0.1,
	}
}

// TextChunk represents a chunk of text with metadata
type TextChunk struct {
	Content    string            `json:"content"`
	StartLine  int               `json:"start_line"`
	EndLine    int               `json:"end_line"`
	ChunkIndex int               `json:"chunk_index"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// Chunker handles text chunking operations
type Chunker struct {
	options ChunkOptions
}

// NewChunker creates a new text chunker with the given options
func NewChunker(options ChunkOptions) *Chunker {
	// Calculate overlap size from ratio if not explicitly set
	if options.OverlapSize == 0 && options.OverlapRatio > 0 {
		options.OverlapSize = int(float64(options.MaxSize) * options.OverlapRatio)
	}

	return &Chunker{
		options: options,
	}
}

// ChunkText splits text into chunks according to the configured strategy
func (c *Chunker) ChunkText(text string) ([]TextChunk, error) {
	switch c.options.Strategy {
	case ChunkByLines:
		return c.chunkByLines(text)
	case ChunkByTokens:
		return c.chunkByTokens(text)
	case ChunkBySemanticBoundaries:
		return c.chunkBySemanticBoundaries(text)
	default:
		return nil, fmt.Errorf("unsupported chunk strategy: %d", c.options.Strategy)
	}
}

// chunkByLines splits text into chunks based on line count
func (c *Chunker) chunkByLines(text string) ([]TextChunk, error) {
	lines := strings.Split(text, "\n")
	var chunks []TextChunk

	chunkIndex := 0
	for i := 0; i < len(lines); {
		// Calculate chunk end
		end := i + c.options.MaxSize
		if end > len(lines) {
			end = len(lines)
		}

		// Create chunk
		chunkLines := lines[i:end]
		chunk := TextChunk{
			Content:    strings.Join(chunkLines, "\n"),
			StartLine:  i + 1, // 1-indexed
			EndLine:    end,
			ChunkIndex: chunkIndex,
		}
		chunks = append(chunks, chunk)

		// Move to next chunk with overlap
		i = end - c.options.OverlapSize
		if i <= 0 || i >= len(lines) {
			break
		}
		chunkIndex++
	}

	return chunks, nil
}

// chunkByTokens splits text into chunks based on approximate token count
func (c *Chunker) chunkByTokens(text string) ([]TextChunk, error) {
	lines := strings.Split(text, "\n")

	var chunks []TextChunk
	chunkIndex := 0
	currentTokens := 0
	currentLines := []string{}
	startLine := 1

	for lineNum, line := range lines {
		lineTokens := len(strings.Fields(line))

		// If adding this line would exceed max size, create a chunk
		if currentTokens+lineTokens > c.options.MaxSize && len(currentLines) > 0 {
			chunk := TextChunk{
				Content:    strings.Join(currentLines, "\n"),
				StartLine:  startLine,
				EndLine:    lineNum,
				ChunkIndex: chunkIndex,
			}
			chunks = append(chunks, chunk)

			// Start new chunk with overlap
			overlapLines := c.options.OverlapSize
			if overlapLines > len(currentLines) {
				overlapLines = len(currentLines)
			}

			if overlapLines > 0 {
				currentLines = currentLines[len(currentLines)-overlapLines:]
				startLine = lineNum - overlapLines + 1
				currentTokens = 0
				for _, ol := range currentLines {
					currentTokens += len(strings.Fields(ol))
				}
			} else {
				currentLines = []string{}
				startLine = lineNum + 1
				currentTokens = 0
			}
			chunkIndex++
		}

		currentLines = append(currentLines, line)
		currentTokens += lineTokens
	}

	// Add final chunk if there's remaining content
	if len(currentLines) > 0 {
		chunk := TextChunk{
			Content:    strings.Join(currentLines, "\n"),
			StartLine:  startLine,
			EndLine:    len(lines),
			ChunkIndex: chunkIndex,
		}
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// chunkBySemanticBoundaries splits text at natural code boundaries
func (c *Chunker) chunkBySemanticBoundaries(text string) ([]TextChunk, error) {
	lines := strings.Split(text, "\n")
	boundaries := c.findSemanticBoundaries(lines)

	var chunks []TextChunk
	chunkIndex := 0
	start := 0

	for _, boundary := range boundaries {
		if boundary-start >= c.options.MaxSize {
			// Create chunk
			chunkLines := lines[start:boundary]
			chunk := TextChunk{
				Content:    strings.Join(chunkLines, "\n"),
				StartLine:  start + 1,
				EndLine:    boundary,
				ChunkIndex: chunkIndex,
			}
			chunks = append(chunks, chunk)

			// Move to next chunk with minimal overlap at boundaries
			start = boundary - min(c.options.OverlapSize, 5)
			if start < 0 {
				start = 0
			}
			chunkIndex++
		}
	}

	// Add final chunk
	if start < len(lines) {
		chunkLines := lines[start:]
		chunk := TextChunk{
			Content:    strings.Join(chunkLines, "\n"),
			StartLine:  start + 1,
			EndLine:    len(lines),
			ChunkIndex: chunkIndex,
		}
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// findSemanticBoundaries identifies natural breaking points in code
func (c *Chunker) findSemanticBoundaries(lines []string) []int {
	var boundaries []int

	// Patterns that indicate good breaking points
	functionPattern := regexp.MustCompile(`^\s*(func|function|def|class|interface|type)\s+`)
	blockEndPattern := regexp.MustCompile(`^\s*}\s*$`)
	commentBlockPattern := regexp.MustCompile(`^\s*(//|#|/\*|\*)`)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Function/class/type definitions
		if functionPattern.MatchString(line) {
			boundaries = append(boundaries, i)
		}

		// End of blocks (closing braces)
		if blockEndPattern.MatchString(line) {
			boundaries = append(boundaries, i+1)
		}

		// Large comment blocks
		if commentBlockPattern.MatchString(line) && len(trimmed) > 50 {
			boundaries = append(boundaries, i)
		}

		// Empty lines that might indicate section breaks
		if trimmed == "" && i > 0 && i < len(lines)-1 {
			prevTrimmed := strings.TrimSpace(lines[i-1])
			nextTrimmed := strings.TrimSpace(lines[i+1])
			if len(prevTrimmed) > 0 && len(nextTrimmed) > 0 {
				boundaries = append(boundaries, i)
			}
		}
	}

	// Remove duplicates and sort
	boundaries = removeDuplicates(boundaries)

	return boundaries
}

// ChunkFile reads a file and chunks its content
func (c *Chunker) ChunkFile(filepath string) ([]TextChunk, error) {
	content, err := readFileContent(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filepath, err)
	}

	chunks, err := c.ChunkText(content)
	if err != nil {
		return nil, err
	}

	// Add file metadata to chunks
	for i := range chunks {
		if chunks[i].Metadata == nil {
			chunks[i].Metadata = make(map[string]string)
		}
		chunks[i].Metadata["file_path"] = filepath
	}

	return chunks, nil
}

// EstimateTokenCount provides a rough estimate of token count for text
func EstimateTokenCount(text string) int {
	// Simple approximation: ~4 characters per token on average
	return utf8.RuneCountInString(text) / 4
}

// Helper functions

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func removeDuplicates(slice []int) []int {
	keys := make(map[int]bool)
	var result []int

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}

	return result
}

func readFileContent(filepath string) (string, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
