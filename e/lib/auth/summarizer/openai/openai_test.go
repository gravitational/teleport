package openai

import (
	"log/slog"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	summarizererrorstypes "github.com/gravitational/teleport/e/lib/auth/summarizer/errors/types"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/prompts"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/structured"
)

func newTestProvider() *InferenceProvider {
	p := &InferenceProvider{
		openAIModelName:   openai.ChatModelGPT5,
		temperature:       1.0,
		client:            &fakeClient{},
		modelResourceName: "test-model",
	}
	p.logger = slog.With(teleport.ComponentKey, "openai", "inference_model", p.modelResourceName)
	return p
}

func TestCondenseForEmbedding(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	provider := newTestProvider()

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
			// The native json_schema response is unparseable, so the request falls back to the prompt-based json_object
			// path, which re-prompts and then returns a BadResponseError once the retries are exhausted.
			name:  "permanently bad JSON returns BadResponseError after retries",
			input: summarizerv1pb.Summary_builder{SessionId: "trigger-bad-json"}.Build(),
			assert: func(t *testing.T, result string, err error) {
				var badResp summarizererrorstypes.BadResponseError
				require.ErrorAs(t, err, &badResp)
				assert.Contains(t, badResp.Message, "after 1 retries")
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

func TestStructuredOutputFallbackAndCache(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	cache := structured.NewSupportCache(clockwork.NewFakeClock(), time.Hour)
	rec := &fakeRecorder{}
	p := &InferenceProvider{
		openAIModelName:       openai.ChatModelGPT5,
		temperature:           1.0,
		client:                &fakeClient{recorder: rec},
		modelResourceName:     "test-model",
		structuredOutputCache: cache,
		nativeSupportKey:      "https://proxy.example\x00gpt-5",
	}
	p.logger = slog.With(teleport.ComponentKey, "openai", "inference_model", p.modelResourceName)

	// First request: the endpoint rejects native json_schema, so the request falls back to the prompt-based
	// json_object path, which succeeds. That success is cached as a "json_schema unsupported" verdict.
	resp, err := p.SummarizeCommand(ctx, "sid", "user", "ubuntu", "native-unsupported please")
	require.NoError(t, err)
	require.Equal(t, "ls -al", resp.Command)
	require.Equal(t, 1, rec.native)
	require.Equal(t, 1, rec.prompt)

	// Second request: the cached verdict skips the wasted native attempt and goes straight to json_object.
	resp, err = p.SummarizeCommand(ctx, "sid", "user", "ubuntu", "native-unsupported again")
	require.NoError(t, err)
	require.Equal(t, "ls -al", resp.Command)
	require.Equal(t, 1, rec.native) // unchanged: native no longer attempted
	require.Equal(t, 2, rec.prompt)
}

func TestSummarizeCommand(t *testing.T) {
	ctx := t.Context()

	provider := &InferenceProvider{
		openAIModelName:   openai.ChatModelGPT5,
		temperature:       1.0,
		client:            &fakeClient{},
		modelResourceName: "test-model",
	}

	provider.logger = slog.With(teleport.ComponentKey, "openai", "inference_model", provider.modelResourceName)

	testCases := []struct {
		name    string
		content string
		assert  func(t *testing.T, resp *schema.CommandAnalysis, err error)
	}{
		{
			name:    "typical case",
			content: "json response for command analysis",
			assert: func(t *testing.T, resp *schema.CommandAnalysis, err error) {
				require.NoError(t, err)
				assert.Equal(t, "ls -al", resp.Command)
				assert.Equal(t, "file_operation", resp.Category)
			},
		},
		{
			name:    "empty response",
			content: "no choices",
			assert: func(t *testing.T, resp *schema.CommandAnalysis, err error) {
				assert.ErrorIs(t, err, summarizererrorstypes.BadResponseError{
					Message: "model returned no choices",
				})
			},
		},
		{
			name:    "length limit exceeded",
			content: "make the output too long",
			assert: func(t *testing.T, resp *schema.CommandAnalysis, err error) {
				assert.ErrorIs(t, err, &trace.LimitExceededError{
					Message: "model response length limit exceeded",
				})
			},
		},
		{
			name:    "bad response",
			content: "cause an error",
			assert: func(t *testing.T, resp *schema.CommandAnalysis, err error) {
				assert.ErrorIs(t, err, summarizererrorstypes.BadResponseError{
					Message: "model returned unexpected finish reason: \"content_filter\"",
				})
			},
		},
	}

	for _, tc := range testCases {
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
	provider := newTestProvider()

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
	provider := newTestProvider()

	systemPrompt := prompts.ScreenshotsSynthesisPrompt
	prompt := "## Desktop Session Events\n\n### Event 1\n- **Time**: 0:00 - 0:05\n"

	resp, err := provider.SummarizeDesktopSession(ctx, "2bce7245-6508-43e0-ab31-b1a2dcec714e", systemPrompt, prompt)
	require.NoError(t, err)
	require.NotNil(t, resp)

	require.Equal(t, "low", resp.RiskLevel)
	require.Equal(t, 15, resp.RiskScore)
	require.False(t, resp.CompromiseIndicators)
}
