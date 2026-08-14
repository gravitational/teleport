package summarizer

import (
	"context"
	"errors"
	"strings"
	"sync"
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

	result, err := summarizeReconstructedCommand(ctx, sessionID, &provider, pool, cmd, "testuser", "ubuntu", "", "")
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

	result, err := summarizeReconstructedCommand(ctx, sessionID, &provider, pool, cmd, "testuser", "ubuntu", "", "")
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

	result, err := summarizeReconstructedCommand(ctx, sessionID, &provider, pool, cmd, "testuser", "ubuntu", "", "")
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

	result, err := summarizeReconstructedCommand(ctx, sessionID, &provider, pool, cmd, "testuser", "ubuntu", "", "")
	require.NoError(t, err)

	require.Empty(t, result.Command)
	require.Equal(t, "Empty command", result.ShortDescription)
	require.Equal(t, "low", result.RiskLevel)
	require.Equal(t, 0, result.RiskScore)
	require.False(t, result.Success)
}

func TestSummarizeReconstructedCommand_SingleChunk_PrependsPrefixes(t *testing.T) {
	ctx := t.Context()
	provider := &recordingProvider{}
	pool := newWorkerPool(5)

	cmd := &mockCommand{chunks: []string{"analyze this"}}

	_, err := summarizeReconstructedCommand(ctx, "sid", provider, pool, cmd, "u", "l",
		"SESSION METADATA:\nServer hostname: prod\n\n",
		"PRIOR COMMAND CONTEXT:\n================\n1. Command: ls\n================\n\n",
	)
	require.NoError(t, err)

	require.Len(t, provider.prompts, 1)
	require.True(t, strings.HasPrefix(provider.prompts[0], "SESSION METADATA:"))
	require.Contains(t, provider.prompts[0], "PRIOR COMMAND CONTEXT:")
	require.True(t, strings.HasSuffix(provider.prompts[0], "analyze this"))
}

func TestSummarizeReconstructedCommand_MultiChunk_PrependsPrefixes(t *testing.T) {
	ctx := t.Context()
	provider := &recordingProvider{}
	pool := newWorkerPool(5)

	cmd := &mockCommand{chunks: []string{"chunk-a", "chunk-b"}}
	metadata := "SESSION METADATA:\nServer hostname: prod\n\n"
	trail := "PRIOR COMMAND CONTEXT:\n================\n1. Command: ls\n================\n\n"

	_, err := summarizeReconstructedCommand(ctx, "sid", provider, pool, cmd, "u", "l", metadata, trail)
	require.NoError(t, err)

	// 2 individual chunk calls + 1 synthesis call = 3 total.
	require.Len(t, provider.prompts, 3)

	// Individual chunk prompts should have metadata and no context trail.
	for _, p := range provider.prompts[:2] {
		require.True(t, strings.HasPrefix(p, "SESSION METADATA:"), "chunk prompt should start with session metadata")
		require.NotContains(t, p, "PRIOR COMMAND CONTEXT:", "chunk prompt should not contain context trail")
	}

	// Synthesis prompt should have both metadata and context trail.
	synthPrompt := provider.prompts[2]

	require.True(t, strings.HasPrefix(synthPrompt, "SESSION METADATA:"), "synthesis prompt should start with session metadata")
	require.Contains(t, synthPrompt, "PRIOR COMMAND CONTEXT:", "synthesis prompt should contain context trail")
	require.Contains(t, synthPrompt, "Synthesize", "synthesis prompt should contain synthesis instructions")
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

func (m *mockCommand) RawInput() string {
	if len(m.chunks) > 0 {
		return m.chunks[0]
	}
	return ""
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

// recordingProvider captures all prompts sent to SummarizeCommand for assertion.
type recordingProvider struct {
	mu      sync.Mutex
	prompts []string
}

func (r *recordingProvider) SummarizeCommand(
	_ context.Context,
	_ session.ID,
	_, _,
	prompt string,
) (*schema.CommandAnalysis, error) {
	r.mu.Lock()
	r.prompts = append(r.prompts, prompt)
	r.mu.Unlock()

	return &schema.CommandAnalysis{
		Command:          "test",
		ShortDescription: "test command",
		RiskLevel:        "low",
		RiskScore:        5,
		Success:          true,
	}, nil
}
