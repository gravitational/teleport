package bedrock

import (
	"io"
	"strings"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	summarizererrors "github.com/gravitational/teleport/e/lib/auth/summarizer/errors"
)

func TestInferenceProvider(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	provider, err := NewProvider(ctx, ProviderConfig{
		Spec: &summarizerv1pb.BedrockProvider{
			Region:         "us-east-1",
			BedrockModelId: "anthropic.claude-3-haiku-20240307-v1:0",
		},
		ModelResourceName: "claude",
		ClientFactory:     &FakeClientFactory{Clock: clockwork.NewFakeClock()},
	})
	require.NoError(t, err)

	cases := []struct {
		name    string
		content string
		assert  func(t *testing.T, resp string, err error)
	}{
		{
			name:    "typical case",
			content: "ps aux",
			assert: func(t *testing.T, resp string, err error) {
				require.NoError(t, err)
				assert.Equal(t, "The user wrote: ps aux", resp)
			},
		},
		{
			name:    "multiple blocks",
			content: "respond with multiple content blocks",
			assert: func(t *testing.T, resp string, err error) {
				require.NoError(t, err)
				assert.Equal(t, "block 1, block 2", resp)
			},
		},
		{
			name:    "empty response (no blocks at all)",
			content: "no choices",
			assert: func(t *testing.T, resp string, err error) {
				assert.ErrorIs(t, err, summarizererrors.BadResponseError{
					Message: "model returned a message without content",
				})
			},
		},
		{
			name:    "empty response (blocks without content)",
			content: "respond with multiple empty content blocks",
			assert: func(t *testing.T, resp string, err error) {
				assert.ErrorIs(t, err, summarizererrors.BadResponseError{
					Message: "model returned a message without content",
				})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content := io.NopCloser(strings.NewReader(tc.content))
			resp, err := provider.Summarize(
				ctx, "2bce7245-6508-43e0-ab31-b1a2dcec714e", "Analyze this terminal session", content,
			)
			tc.assert(t, resp, err)
		})
	}
}
