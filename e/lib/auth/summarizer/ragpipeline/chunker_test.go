package ragpipeline

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdjustToSentenceBoundary(t *testing.T) {
	c := &chunker{}

	buildText := func(prefixLen int, ending, suffix string) string {
		return strings.Repeat("a", prefixLen) + ending + suffix
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			// total = 100: searchStart = 80; boundary at pos 80 .
			name:  "period-space boundary in last 20%",
			input: buildText(80, ". ", "trailing text here!"),
			want:  buildText(80, ". ", ""),
		},
		{
			// total = 100: boundary at pos 80
			name:  "exclamation-space boundary in last 20%",
			input: buildText(80, "! ", "trailing text here."),
			want:  buildText(80, "! ", ""),
		},
		{
			// total = 100: boundary at pos 80
			name:  "question-space boundary in last 20%",
			input: buildText(80, "? ", "trailing text here."),
			want:  buildText(80, "? ", ""),
		},
		{
			// total = 100: boundary at pos 80
			name:  "period-newline boundary in last 20%",
			input: buildText(80, ".\n", "trailing text here."),
			want:  buildText(80, ".\n", ""),
		},
		{
			// total = 100: boundary at pos 80
			name:  "picks last boundary when multiple in search window",
			input: buildText(80, ". ", "first. second"),
			want:  buildText(80, ". ", "first. "),
		},
		{
			// Boundary at position 20 in a 100-char string: searchStart=80, not found.
			name:  "boundary only in first 80% is ignored",
			input: buildText(20, ". ", strings.Repeat("b", 78)),
			want:  buildText(20, ". ", strings.Repeat("b", 78)),
		},
		{
			name:  "no sentence boundary returns text unchanged",
			input: strings.Repeat("a", 50),
			want:  strings.Repeat("a", 50),
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "single character",
			input: "x",
			want:  "x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.adjustToSentenceBoundary(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// teleportSentence is a 35-token sentence used across chunking tests.
const teleportSentence = "Teleport is the AI Infrastructure Identity Company, modernizing identity, " +
	"access, and policy for infrastructure, improving engineering velocity and resiliency of critical " +
	"infrastructure against human factors and/or compromise."

func TestChunkText(t *testing.T) {
	tests := []struct {
		name  string
		cfg   ChunkerConfig
		check func(t *testing.T, chunks []string)
	}{
		{
			// 35 tokens fits well within 800, so the sentence is returned as-is.
			name: "fits in one chunk",
			cfg:  ChunkerConfig{MaxTokensPerChunk: 800, OverlapTokens: 200},
			check: func(t *testing.T, chunks []string) {
				require.Equal(t, []string{teleportSentence}, chunks)
			},
		},
		{
			// 35-token sentence, max=20, overlap=5: the single sentence has no internal
			// sentence boundaries so adjustToSentenceBoundary leaves chunks unmodified.
			// chunk[1] reaches totalTokens so the loop terminates without a tail duplicate.
			name: "long text is split into expected chunks",
			cfg:  ChunkerConfig{MaxTokensPerChunk: 20, OverlapTokens: 5},
			check: func(t *testing.T, chunks []string) {
				require.Equal(t, []string{
					"Teleport is the AI Infrastructure Identity Company, modernizing identity, access, and policy for infrastructure, improving",
					" policy for infrastructure, improving engineering velocity and resiliency of critical infrastructure against human factors and/or compromise.",
				}, chunks)
			},
		},
		{
			// 35-token sentence, max=15, overlap=5: each chunk[n+1] re-opens with tokens
			// from the tail of chunk[n]'s window, making the overlapping phrases visible.
			// chunk[2] reaches totalTokens so the loop terminates without a tail duplicate.
			name: "adjacent chunks share text from the overlap window",
			cfg:  ChunkerConfig{MaxTokensPerChunk: 15, OverlapTokens: 5},
			check: func(t *testing.T, chunks []string) {
				require.Equal(t, []string{
					"Teleport is the AI Infrastructure Identity Company, modernizing identity, access, and",
					" identity, access, and policy for infrastructure, improving engineering velocity and resiliency",
					" engineering velocity and resiliency of critical infrastructure against human factors and/or compromise.",
				}, chunks)

				// Every chunk[n+1] starts from 5 tokens before chunk[n]'s window closed,
				// so those 5 tokens decode to a phrase present in both neighbors.
				overlapPhrases := []string{
					"access, and",    // tail of chunk[0] window, reopens chunk[1]
					"and resiliency", // tail of chunk[1] window, reopens chunk[2]
				}
				for i, phrase := range overlapPhrases {
					assert.Contains(t, chunks[i], phrase, "chunk[%d] should contain overlap phrase %q", i, phrase)
					assert.Contains(t, chunks[i+1], phrase, "chunk[%d] should contain overlap phrase %q", i+1, phrase)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := newChunkerWithConfig(tt.cfg)
			require.NoError(t, err)
			chunks, err := c.chunkText(teleportSentence)
			require.NoError(t, err)
			tt.check(t, chunks)
		})
	}
}
