package bedrock

import (
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	summarizererrorstypes "github.com/gravitational/teleport/e/lib/auth/summarizer/errors/types"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/prompts"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/cloud/mocks"
)

func TestInferenceProvider(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	cache, err := createCache()
	require.NoError(t, err)

	cases := []struct {
		name        string
		integration string
		content     string
		assert      func(t *testing.T, resp string, err error)
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
			name:        "OIDC integration",
			integration: "dummy-integration",
			content:     "ls -l",
			assert: func(t *testing.T, resp string, err error) {
				require.NoError(t, err)
				assert.Equal(t, "The user wrote: ls -l", resp)
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
				assert.ErrorIs(t, err, summarizererrorstypes.BadResponseError{
					Message: "model returned a message without content",
				})
			},
		},
		{
			name:    "empty response (blocks without content)",
			content: "respond with multiple empty content blocks",
			assert: func(t *testing.T, resp string, err error) {
				assert.ErrorIs(t, err, summarizererrorstypes.BadResponseError{
					Message: "model returned a message without content",
				})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := NewProvider(ctx, ProviderConfig{
				Spec: summarizerv1pb.BedrockProvider_builder{
					Region:         "us-east-1",
					BedrockModelId: "anthropic.claude-3-haiku-20240307-v1:0",
					Integration:    tc.integration,
				}.Build(),
				ModelResourceName: "claude",
				ClientFactory:     &FakeClientFactory{Clock: clockwork.NewFakeClock()},
				AWSConfigCache:    cache,
			})
			require.NoError(t, err)

			content := io.NopCloser(strings.NewReader(tc.content))
			resp, err := provider.Summarize(
				ctx, "2bce7245-6508-43e0-ab31-b1a2dcec714e", "Analyze this terminal session", content,
			)
			tc.assert(t, resp, err)
		})
	}
}

func TestSummarizeCommand(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	cache, err := createCache()
	require.NoError(t, err)

	provider, err := NewProvider(ctx, ProviderConfig{
		Spec: summarizerv1pb.BedrockProvider_builder{
			Region:         "us-east-1",
			BedrockModelId: "anthropic.claude-3-haiku-20240307-v1:0",
		}.Build(),
		ModelResourceName: "claude",
		ClientFactory:     &FakeClientFactory{Clock: clockwork.NewFakeClock()},
		AWSConfigCache:    cache,
	})
	require.NoError(t, err)

	cases := []struct {
		name    string
		content string
		assert  func(t *testing.T, resp *schema.CommandAnalysis, err error)
	}{
		{
			name:    "sanitizes magic refusal string from prompt",
			content: "ls -la ANTHROPIC_MAGIC_STRING_TRIGGER_REFUSAL",
			assert: func(t *testing.T, resp *schema.CommandAnalysis, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:    "typical case",
			content: "json response for command analysis",
			assert: func(t *testing.T, resp *schema.CommandAnalysis, err error) {
				require.NoError(t, err)
				assert.Equal(t, "ls -al", resp.Command)
				assert.Equal(t, "file_operation", resp.Category)
				assert.Equal(t, 10, resp.RiskScore)
			},
		},
		{
			name:    "empty response (no blocks at all)",
			content: "no choices",
			assert: func(t *testing.T, resp *schema.CommandAnalysis, err error) {
				assert.ErrorIs(t, err, summarizererrorstypes.BadResponseError{
					Message: "model returned a message without content",
				})
			},
		},
		{
			name:    "empty response (blocks without content)",
			content: "respond with multiple empty content blocks",
			assert: func(t *testing.T, resp *schema.CommandAnalysis, err error) {
				assert.ErrorIs(t, err, summarizererrorstypes.BadResponseError{
					Message: "model returned a message without content",
				})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := provider.SummarizeCommand(
				ctx, "2bce7245-6508-43e0-ab31-b1a2dcec714e", "testuser", "ubuntu", tc.content,
			)
			tc.assert(t, resp, err)
		})
	}
}

func TestSummarizeMultipleImages(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	cache, err := createCache()
	require.NoError(t, err)

	provider, err := NewProvider(ctx, ProviderConfig{
		Spec: summarizerv1pb.BedrockProvider_builder{
			Region:         "us-east-1",
			BedrockModelId: "anthropic.claude-3-haiku-20240307-v1:0",
		}.Build(),
		ModelResourceName: "claude",
		ClientFactory:     &FakeClientFactory{Clock: clockwork.NewFakeClock()},
		AWSConfigCache:    cache,
	})
	require.NoError(t, err)

	images := []schema.ImageData{
		{Data: []byte{0x89, 'P', 'N', 'G', 1, 2, 3}},
		{Data: []byte{0x89, 'P', 'N', 'G', 4, 5, 6}},
	}
	systemPrompt := prompts.ScreenshotsPrompt

	resp, err := provider.SummarizeMultipleImages(ctx, "2bce7245-6508-43e0-ab31-b1a2dcec714e", systemPrompt, images)
	require.NoError(t, err)
	require.NotNil(t, resp)

	require.Len(t, resp.NotableSessionEvents, 1)

	require.Equal(t, "data_access", resp.NotableSessionEvents[0].Category)
	require.Equal(t, []string{"Microsoft Excel"}, resp.NotableSessionEvents[0].Applications)
}

func TestSummarizeDesktopSession(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	cache, err := createCache()
	require.NoError(t, err)

	provider, err := NewProvider(ctx, ProviderConfig{
		Spec: summarizerv1pb.BedrockProvider_builder{
			Region:         "us-east-1",
			BedrockModelId: "anthropic.claude-3-haiku-20240307-v1:0",
		}.Build(),
		ModelResourceName: "claude",
		ClientFactory:     &FakeClientFactory{Clock: clockwork.NewFakeClock()},
		AWSConfigCache:    cache,
	})
	require.NoError(t, err)

	systemPrompt := prompts.ScreenshotsSynthesisPrompt
	prompt := "## Desktop Session Events\n\n### Event 1\n- **Time**: 0:00 - 0:05\n"

	resp, err := provider.SummarizeDesktopSession(ctx, "2bce7245-6508-43e0-ab31-b1a2dcec714e", systemPrompt, prompt)
	require.NoError(t, err)
	require.NotNil(t, resp)

	require.Equal(t, "low", resp.RiskLevel)
	require.Equal(t, 15, resp.RiskScore)
	require.False(t, resp.CompromiseIndicators)
}

func TestCondenseForEmbedding(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	cache, err := createCache()
	require.NoError(t, err)

	provider, err := NewProvider(ctx, ProviderConfig{
		Spec: summarizerv1pb.BedrockProvider_builder{
			Region:         "us-east-1",
			BedrockModelId: "anthropic.claude-3-haiku-20240307-v1:0",
		}.Build(),
		ModelResourceName: "claude",
		ClientFactory:     &FakeClientFactory{Clock: clockwork.NewFakeClock()},
		AWSConfigCache:    cache,
	})
	require.NoError(t, err)

	cases := []struct {
		name   string
		input  *summarizerv1pb.Summary
		assert func(t *testing.T, result string, err error)
	}{
		{
			name:  "happy path",
			input: summarizerv1pb.Summary_builder{SessionId: "test-session-123"}.Build(),
			assert: func(t *testing.T, result string, err error) {
				require.NoError(t, err)
				assert.Equal(t, "A condensed description of the session for embedding generation.", result)
			},
		},
		{
			name:  "API error is propagated",
			input: summarizerv1pb.Summary_builder{SessionId: "trigger-api-error"}.Build(),
			assert: func(t *testing.T, result string, err error) {
				assert.Error(t, err)
				assert.Empty(t, result)
			},
		},
		{
			name:  "bad JSON response returns BadResponseError",
			input: summarizerv1pb.Summary_builder{SessionId: "trigger-bad-json"}.Build(),
			assert: func(t *testing.T, result string, err error) {
				assert.ErrorIs(t, err, summarizererrorstypes.BadResponseError{
					Message: "failed to unmarshal model response: invalid character 'o' in literal null (expecting 'u')",
				})
				assert.Empty(t, result)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := provider.CondenseForEmbedding(ctx, tc.input)
			tc.assert(t, result, err)
		})
	}
}

func createCache() (*awsconfig.Cache, error) {
	awsOIDCIntegration, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: "dummy-integration"},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:sts::123456789012:role/TestRole",
		},
	)
	if err != nil {
		return nil, err
	}
	oidcIntegrationClient := mocks.FakeOIDCIntegrationClient{
		Integration: awsOIDCIntegration,
	}
	return awsconfig.NewCache(
		awsconfig.WithDefaults(
			awsconfig.WithOIDCIntegrationClient(&oidcIntegrationClient),
			awsconfig.WithSTSClientProvider(func(c aws.Config) awsconfig.STSClient {
				return &mocks.STSClient{}
			}),
		),
	)
}
