package bedrock

import (
	"testing"
	"time"

	"github.com/aws/smithy-go"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	summarizererrorstypes "github.com/gravitational/teleport/e/lib/auth/summarizer/errors/types"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/structured"
)

const (
	supportedModelID   = "anthropic.claude-opus-4-6-v1"
	unsupportedModelID = "anthropic.claude-3-haiku-20240307-v1:0"
	unknownModelID     = "deepseek.v3-v1:0"
)

func TestStructuredOutputPathSelection(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	t.Run("supported model uses native API", func(t *testing.T) {
		rec := &fakeRecorder{}
		p := newStructuredTestProvider(t, supportedModelID, rec)

		resp, err := p.SummarizeCommand(ctx, "sid", "user", "ubuntu", "list files")
		require.NoError(t, err)

		require.Equal(t, "test-command", resp.Command)
		require.Equal(t, 1, rec.nativeCalls())
		require.Equal(t, 0, rec.promptCalls())
	})

	t.Run("known-unsupported model uses prompt path directly", func(t *testing.T) {
		rec := &fakeRecorder{}
		p := newStructuredTestProvider(t, unsupportedModelID, rec)

		resp, err := p.SummarizeCommand(ctx, "sid", "user", "ubuntu", "list files")
		require.NoError(t, err)

		require.Equal(t, "test-command", resp.Command)
		require.Equal(t, 0, rec.nativeCalls())
		require.Equal(t, 1, rec.promptCalls())
	})
}

func TestStructuredOutputNativeFallback(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	t.Run("falls back to prompt on a recoverable native error", func(t *testing.T) {
		rec := &fakeRecorder{}
		p := newStructuredTestProvider(t, supportedModelID, rec)

		resp, err := p.SummarizeCommand(ctx, "sid", "user", "ubuntu", "native-validation-fail please")
		require.NoError(t, err)

		require.Equal(t, "test-command", resp.Command) // produced by the prompt fallback
		require.Equal(t, 1, rec.nativeCalls())
		require.Equal(t, 1, rec.promptCalls())
	})

	t.Run("surfaces a transient native error without falling back", func(t *testing.T) {
		rec := &fakeRecorder{}
		p := newStructuredTestProvider(t, supportedModelID, rec)

		resp, err := p.SummarizeCommand(ctx, "sid", "user", "ubuntu", "native-throttle now")
		require.Error(t, err)
		require.Nil(t, resp)

		var apiErr smithy.APIError
		require.ErrorAs(t, err, &apiErr)

		require.Equal(t, "ThrottlingException", apiErr.ErrorCode())
		require.Equal(t, 1, rec.nativeCalls())
		require.Equal(t, 0, rec.promptCalls()) // no fallback for transient errors
	})
}

func TestStructuredOutputPromptResiliency(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	t.Run("recovers JSON wrapped in Markdown and prose", func(t *testing.T) {
		rec := &fakeRecorder{}
		p := newStructuredTestProvider(t, unsupportedModelID, rec)

		resp, err := p.SummarizeCommand(ctx, "sid", "user", "ubuntu", "prompt-prose-wrap")
		require.NoError(t, err)

		require.Equal(t, "ls -al", resp.Command)
		require.Equal(t, 1, rec.promptCalls())
	})

	t.Run("re-prompts once and recovers", func(t *testing.T) {
		rec := &fakeRecorder{}
		p := newStructuredTestProvider(t, unsupportedModelID, rec)

		resp, err := p.SummarizeCommand(ctx, "sid", "user", "ubuntu", "prompt-retry-recover")
		require.NoError(t, err)

		require.Equal(t, "ls -al", resp.Command)
		require.Equal(t, 2, rec.promptCalls()) // initial attempt + one re-prompt
	})

	t.Run("returns BadResponseError after exhausting re-prompts", func(t *testing.T) {
		rec := &fakeRecorder{}

		p := newStructuredTestProvider(t, unsupportedModelID, rec)
		resp, err := p.SummarizeCommand(ctx, "sid", "user", "ubuntu", "prompt-retry-exhaust")
		require.Error(t, err)
		require.Nil(t, resp)

		var badResp summarizererrorstypes.BadResponseError
		require.ErrorAs(t, err, &badResp)

		require.Contains(t, badResp.Message, "after 1 retries")
		require.Equal(t, 2, rec.promptCalls())
	})

	t.Run("rejects a decodable response that violates the schema", func(t *testing.T) {
		rec := &fakeRecorder{}

		p := newStructuredTestProvider(t, unsupportedModelID, rec)
		// The model returns {"command":"ls -al"}: valid JSON that unmarshals into the struct but omits required schema
		// fields. The native path would have rejected it server-side, so the prompt path must too.
		resp, err := p.SummarizeCommand(ctx, "sid", "user", "ubuntu", "prompt-schema-violation")
		require.Error(t, err)
		require.Nil(t, resp)

		var badResp summarizererrorstypes.BadResponseError
		require.ErrorAs(t, err, &badResp)

		require.Equal(t, 2, rec.promptCalls()) // initial attempt + one re-prompt, both schema-invalid
	})
}

func TestStructuredOutputUnknownModelMemoization(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	t.Run("tries native once then remembers it is unsupported", func(t *testing.T) {
		rec := &fakeRecorder{}
		p := newStructuredTestProvider(t, unknownModelID, rec)

		// First request optimistically tries native, fails, and falls back.
		resp, err := p.SummarizeCommand(ctx, "sid", "user", "ubuntu", "native-validation-fail one")
		require.NoError(t, err)

		require.Equal(t, "test-command", resp.Command)
		require.Equal(t, 1, rec.nativeCalls())
		require.Equal(t, 1, rec.promptCalls())

		// Second request skips native entirely now.
		resp, err = p.SummarizeCommand(ctx, "sid", "user", "ubuntu", "native-validation-fail two")
		require.NoError(t, err)

		require.Equal(t, "test-command", resp.Command)
		require.Equal(t, 1, rec.nativeCalls()) // unchanged: native no longer attempted
		require.Equal(t, 2, rec.promptCalls())
	})

	t.Run("keeps using native when it succeeds", func(t *testing.T) {
		rec := &fakeRecorder{}
		p := newStructuredTestProvider(t, unknownModelID, rec)

		for range 2 {
			resp, err := p.SummarizeCommand(ctx, "sid", "user", "ubuntu", "list files")
			require.NoError(t, err)

			require.Equal(t, "test-command", resp.Command)
		}

		require.Equal(t, 2, rec.nativeCalls())
		require.Equal(t, 0, rec.promptCalls())
	})
}

func TestStructuredOutputCacheAcrossProviders(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	clock := clockwork.NewFakeClock()
	cache := structured.NewSupportCache(clock, time.Hour)

	// First provider (one summarization job): the unknown model optimistically tries native, fails, falls back, and
	// records the verdict in the shared cache.
	rec1 := &fakeRecorder{}
	p1 := newStructuredTestProviderWithCache(t, unknownModelID, rec1, cache)
	_, err := p1.SummarizeCommand(ctx, "sid", "user", "ubuntu", "native-validation-fail one")
	require.NoError(t, err)

	require.Equal(t, 1, rec1.nativeCalls())
	require.Equal(t, 1, rec1.promptCalls())

	// Second provider (next job), same model + shared cache: native is skipped entirely thanks to the cached verdict, so
	// no wasted native attempt.
	rec2 := &fakeRecorder{}
	p2 := newStructuredTestProviderWithCache(t, unknownModelID, rec2, cache)
	_, err = p2.SummarizeCommand(ctx, "sid", "user", "ubuntu", "native-validation-fail two")
	require.NoError(t, err)

	require.Equal(t, 0, rec2.nativeCalls())
	require.Equal(t, 1, rec2.promptCalls())

	// After the TTL elapses, the verdict expires and native is re-probed.
	clock.Advance(2 * time.Hour)
	rec3 := &fakeRecorder{}
	p3 := newStructuredTestProviderWithCache(t, unknownModelID, rec3, cache)
	_, err = p3.SummarizeCommand(ctx, "sid", "user", "ubuntu", "native-validation-fail three")
	require.NoError(t, err)

	require.Equal(t, 1, rec3.nativeCalls())
	require.Equal(t, 1, rec3.promptCalls())
}

func TestClassifyStructuredOutput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		modelID string
		want    structured.NativeSupport
	}{
		// Supported Anthropic families, across base IDs, geo/global inference profiles, and date/version-suffixed IDs.
		{"opus 4.6 base", "anthropic.claude-opus-4-6-v1", structured.SupportYes},
		{"opus 4.6 us profile", "us.anthropic.claude-opus-4-6-v1", structured.SupportYes},
		{"opus 4.6 global profile", "global.anthropic.claude-opus-4-6-v1", structured.SupportYes},
		{"sonnet 4.6 base", "anthropic.claude-sonnet-4-6", structured.SupportYes},
		{"sonnet 4.6 eu profile", "eu.anthropic.claude-sonnet-4-6", structured.SupportYes},
		{"opus 4.5", "anthropic.claude-opus-4-5-20250101-v1:0", structured.SupportYes},
		{"sonnet 4.5", "anthropic.claude-sonnet-4-5-20250101-v1:0", structured.SupportYes},
		{"haiku 4.5", "anthropic.claude-haiku-4-5-20251001-v1:0", structured.SupportYes},
		{"opus 4.6 ARN", "arn:aws:bedrock:us-east-1:123456789012:inference-profile/us.anthropic.claude-opus-4-6-v1", structured.SupportYes},
		{"mixed case", "US.Anthropic.Claude-Opus-4-6-V1", structured.SupportYes},

		// Known-unsupported families.
		{"opus 4.7", "anthropic.claude-opus-4-7", structured.SupportNo},
		{"opus 4.8 us profile", "us.anthropic.claude-opus-4-8", structured.SupportNo},
		{"opus 4.1", "anthropic.claude-opus-4-1-20250805-v1:0", structured.SupportNo},
		{"claude 3 haiku", "anthropic.claude-3-haiku-20240307-v1:0", structured.SupportNo},
		{"claude 3.5 sonnet", "anthropic.claude-3-5-sonnet-20241022-v2:0", structured.SupportNo},
		{"nova lite", "amazon.nova-lite-v1:0", structured.SupportNo},
		{"nova pro us profile", "us.amazon.nova-pro-v1:0", structured.SupportNo},
		{"titan text", "amazon.titan-text-express-v1", structured.SupportNo},
		{"llama", "meta.llama3-70b-instruct-v1:0", structured.SupportNo},

		// Unknown models: not in either list. These rely on the optimistic native attempt plus fallback.
		{"deepseek", "deepseek.v3-v1:0", structured.SupportUnknown},
		{"qwen3", "qwen.qwen3-32b-v1:0", structured.SupportUnknown},
		{"claude 4.0 original", "anthropic.claude-opus-4-20250514-v1:0", structured.SupportUnknown},

		// Application inference profile ARNs are opaque and do not contain the model name, so they cannot be classified and
		// rely on the optimistic native attempt plus fallback.
		{"application inference profile ARN", "arn:aws:bedrock:us-east-1:123456789012:application-inference-profile/a1b2c3d4e5f6", structured.SupportUnknown},
		{"empty", "", structured.SupportUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, classifyStructuredOutput(tc.modelID))
		})
	}
}

func TestShouldFallbackToPrompt(t *testing.T) {
	t.Parallel()

	validationErr := &smithy.OperationError{
		ServiceID:     "Bedrock Runtime",
		OperationName: "Converse",
		Err:           &smithy.GenericAPIError{Code: "ValidationException", Message: "output format not supported"},
	}
	throttleErr := &smithy.OperationError{
		ServiceID:     "Bedrock Runtime",
		OperationName: "Converse",
		Err:           &smithy.GenericAPIError{Code: "ThrottlingException", Message: "slow down"},
	}

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"bad response", summarizererrorstypes.BadResponseError{Message: "bad"}, true},
		{"wrapped bad response", trace.Wrap(summarizererrorstypes.BadResponseError{Message: "bad"}), true},
		{"validation exception", validationErr, true},
		{"wrapped validation exception", trace.Wrap(validationErr), true},
		{"throttling", throttleErr, false},
		{"wrapped throttling", trace.Wrap(throttleErr), false},
		{"limit exceeded", trace.LimitExceeded("model response length limit exceeded"), false},
		{"generic", trace.BadParameter("nope"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, shouldFallbackToPrompt(tc.err))
		})
	}
}

// newStructuredTestProvider builds a provider backed by the fake client for the given Bedrock model ID, wiring in a
// recorder so tests can assert which structured output path each request used.
func newStructuredTestProvider(t *testing.T, modelID string, rec *fakeRecorder) *InferenceProvider {
	return newStructuredTestProviderWithCache(t, modelID, rec, nil)
}

// newStructuredTestProviderWithCache is newStructuredTestProvider with a shared structured output support cache, used
// to exercise cross-provider memoization.
func newStructuredTestProviderWithCache(t *testing.T, modelID string, rec *fakeRecorder, soCache *structured.SupportCache) *InferenceProvider {
	t.Helper()
	if soCache == nil {
		soCache = testStructuredOutputCache()
	}

	awsCache, err := createCache()
	require.NoError(t, err)

	provider, err := NewProvider(t.Context(), ProviderConfig{
		Spec: summarizerv1pb.BedrockProvider_builder{
			Region:         "us-east-1",
			BedrockModelId: modelID,
		}.Build(),
		ModelResourceName:     "claude",
		ClientFactory:         &FakeClientFactory{Clock: clockwork.NewFakeClock(), recorder: rec},
		AWSConfigCache:        awsCache,
		StructuredOutputCache: soCache,
	})
	require.NoError(t, err)

	return provider
}
