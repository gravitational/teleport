package summarizer

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema"
	"github.com/gravitational/teleport/lib/session"
)

func TestSummarizeReconstructedCommand_SingleChunk(t *testing.T) {
	ctx := t.Context()

	sessionID := session.ID("test-session-123")

	cmd := &mockCommand{
		chunks: []string{"single command prompt"},
	}

	var provider mockInferenceProvider

	pool := newWorkerPool(5)

	result, err := summarizeReconstructedCommand(ctx, sessionID, &provider, pool, cmd, "testuser", "ubuntu")
	require.NoError(t, err)

	require.Equal(t, "ls -la", result.Command)
	require.Equal(t, "List directory contents", result.ShortDescription)
	require.Equal(t, "low", result.RiskLevel)
	require.Equal(t, 10, result.RiskScore)
	require.True(t, result.Success)
}

func TestSummarizeReconstructedCommand_MultipleChunks(t *testing.T) {
	ctx := t.Context()

	sessionID := session.ID("test-session-456")

	cmd := &mockCommand{
		chunks: []string{"chunk1", "chunk2", "chunk3"},
	}

	var provider mockInferenceProvider

	pool := newWorkerPool(5)

	result, err := summarizeReconstructedCommand(ctx, sessionID, &provider, pool, cmd, "testuser", "ubuntu")
	require.NoError(t, err)

	require.Equal(t, "combined command", result.Command)
	require.Equal(t, "Combined multi-chunk command", result.ShortDescription)
	require.Equal(t, "medium", result.RiskLevel)
	require.Equal(t, 50, result.RiskScore)
	require.True(t, result.Success)
}

func TestSummarizeReconstructedCommand_ErrorFromProvider(t *testing.T) {
	ctx := t.Context()

	sessionID := session.ID("test-session-789")

	cmd := &mockCommand{
		chunks: []string{"error prompt"},
	}

	var provider mockInferenceProvider

	pool := newWorkerPool(5)

	result, err := summarizeReconstructedCommand(ctx, sessionID, &provider, pool, cmd, "testuser", "ubuntu")
	require.ErrorContains(t, err, "provider error")
	require.Nil(t, result)
}

func TestSummarizeReconstructedCommand_EmptyCommand(t *testing.T) {
	ctx := t.Context()

	sessionID := session.ID("test-session-empty")

	cmd := &mockCommand{
		chunks: []string{},
	}

	var provider mockInferenceProvider

	pool := newWorkerPool(5)

	result, err := summarizeReconstructedCommand(ctx, sessionID, &provider, pool, cmd, "testuser", "ubuntu")
	require.NoError(t, err)

	require.Empty(t, result.Command)
	require.Equal(t, "Empty command", result.ShortDescription)
	require.Equal(t, "low", result.RiskLevel)
	require.Equal(t, 0, result.RiskScore)
	require.False(t, result.Success)
}

type mockCommand struct {
	chunks []string
}

func (m *mockCommand) ChunkCount() int {
	return len(m.chunks)
}

func (m *mockCommand) PromptForChunk(index int) string {
	if index < 0 || index >= len(m.chunks) {
		return ""
	}

	return m.chunks[index]
}

func (m *mockCommand) StartOffset() time.Duration {
	return 0
}

func (m *mockCommand) EndOffset() time.Duration {
	return 0
}

type mockInferenceProvider struct{}

func (m *mockInferenceProvider) SummarizeCommand(
	ctx context.Context,
	sessionID session.ID,
	username,
	loginName,
	prompt string,
) (*schema.CommandAnalysis, error) {
	switch prompt {
	case "single command prompt":
		return &schema.CommandAnalysis{
			Command:          "ls -la",
			ShortDescription: "List directory contents",
			RiskLevel:        "low",
			RiskScore:        10,
			Success:          true,
		}, nil
	case "chunk1", "chunk2", "chunk3":
		return &schema.CommandAnalysis{
			Command:          "partial command",
			ShortDescription: "Partial command chunk",
			RiskLevel:        "low",
			RiskScore:        10,
			Success:          true,
		}, nil
	case "error prompt":
		return nil, errors.New("provider error")
	case "":
		return &schema.CommandAnalysis{
			Command:          "",
			ShortDescription: "Empty command",
			RiskLevel:        "low",
			RiskScore:        0,
			Success:          false,
		}, nil
	default:
		if strings.Contains(prompt, "Synthesize") {
			return &schema.CommandAnalysis{
				Command:          "combined command",
				ShortDescription: "Combined multi-chunk command",
				RiskLevel:        "medium",
				RiskScore:        50,
				Success:          true,
			}, nil
		}
		return nil, errors.New("unexpected prompt: " + prompt)
	}
}
