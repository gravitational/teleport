package ragpipeline

import (
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/auth/summarizer/tokenizer"
)

// ChunkerConfig holds configuration for text and command chunking.
type ChunkerConfig struct {
	// MaxTokensPerChunk is the target maximum number of tokens per chunk.
	// Smaller chunks produce more precise embeddings; the parent-child retrieval
	// pattern (+- neighbors stored as parent_chunk_text) provides callers with
	// enough surrounding context for display. Defaults to 512.
	MaxTokensPerChunk int
	// OverlapTokens is the number of tokens to overlap between consecutive
	// chunks for better context preservation. Defaults to 128 (~25% of chunk).
	OverlapTokens int
}

// defaultChunkerConfig returns a chunkerConfig with sensible defaults for RAG.
func defaultChunkerConfig() ChunkerConfig {
	return ChunkerConfig{
		MaxTokensPerChunk: 512,
		OverlapTokens:     128,
	}
}

// chunker splits text and commands into windows that fit within embedding model
// limits while preserving context via overlap.
type chunker struct {
	maxTokensPerChunk int
	overlapTokens     int
}

// newChunkerWithConfig creates a new [chunker] with the specified configuration.
// Returns an error if MaxTokensPerChunk is not positive or if OverlapTokens >=
// MaxTokensPerChunk, either of which would cause a division by zero in the
// capacity hint or prevent forward progress.
func newChunkerWithConfig(cfg ChunkerConfig) (*chunker, error) {
	if cfg.MaxTokensPerChunk <= 0 {
		return nil, trace.BadParameter("MaxTokensPerChunk must be positive, got %d", cfg.MaxTokensPerChunk)
	}
	if cfg.OverlapTokens < 0 {
		return nil, trace.BadParameter("OverlapTokens must be non-negative, got %d", cfg.OverlapTokens)
	}
	if cfg.OverlapTokens >= cfg.MaxTokensPerChunk {
		return nil, trace.BadParameter("OverlapTokens (%d) must be less than MaxTokensPerChunk (%d)", cfg.OverlapTokens, cfg.MaxTokensPerChunk)
	}
	return &chunker{
		maxTokensPerChunk: cfg.MaxTokensPerChunk,
		overlapTokens:     cfg.OverlapTokens,
	}, nil
}

// chunkText splits text into overlapping windows based on token limits and tries
// to end each window on a sentence boundary for better semantic cohesion.
func (c *chunker) chunkText(text string) ([]string, error) {
	if text == "" {
		return nil, nil
	}

	tokens, err := tokenizer.EncodeTokens(text)
	if err != nil {
		return nil, trace.Wrap(err, "failed to encode text for chunking")
	}
	totalTokens := len(tokens)

	// If it fits in one chunk, return as-is
	if totalTokens <= c.maxTokensPerChunk {
		return []string{text}, nil
	}

	chunks := make([]string, 0, totalTokens/(c.maxTokensPerChunk-c.overlapTokens)+1)
	start := 0

	for start < totalTokens {
		end, windowText, err := c.windowText(tokens, start, totalTokens)
		if err != nil {
			return nil, err
		}

		chunks = append(chunks, windowText)

		// All tokens consumed; stop before the overlap calculation would
		// rewind start and emit a near-duplicate tail chunk.
		if end >= totalTokens {
			break
		}

		// Move start forward, accounting for overlap
		newStart := end - c.overlapTokens

		// Prevent infinite loop - ensure we always make forward progress
		if newStart <= start {
			start = end
		} else {
			start = newStart
		}
	}

	return chunks, nil
}

func (c *chunker) windowText(tokens []int, start, totalTokens int) (int, string, error) {
	end := start + c.maxTokensPerChunk
	if end > totalTokens {
		end = totalTokens
	}

	windowTokens := tokens[start:end]
	windowText, err := tokenizer.DecodeTokens(windowTokens)
	if err != nil {
		return 0, "", trace.Wrap(err, "failed to decode tokens for chunking")
	}

	if end < totalTokens {
		// adjustToSentenceBoundary trims by character position, so end (in tokens)
		// is not updated after trimming. The next window's start is computed from
		// the original end, meaning its overlap window may cover tokens excluded
		// from this chunk's text. This is an intentional approximation — the
		// overlap still ensures context continuity between adjacent chunks.
		windowText = c.adjustToSentenceBoundary(windowText)
	}

	return end, windowText, nil
}

// adjustToSentenceBoundary tries to end chunk at a sentence boundary for better
// semantic coherence in RAG retrieval.
func (c *chunker) adjustToSentenceBoundary(text string) string {
	// Look for sentence endings in the last 20% of text
	searchStart := len(text) * 4 / 5
	if searchStart <= 0 {
		return text
	}

	searchText := text[searchStart:]
	sentenceEndings := []string{". ", ".\n", "! ", "!\n", "? ", "?\n"}
	lastBoundary := -1

	for _, ending := range sentenceEndings {
		idx := strings.LastIndex(searchText, ending)
		if idx >= 0 && idx+len(ending) > lastBoundary {
			lastBoundary = idx + len(ending)
		}
	}

	if lastBoundary > 0 {
		return text[:searchStart+lastBoundary]
	}

	return text
}
