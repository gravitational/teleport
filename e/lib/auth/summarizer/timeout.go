package summarizer

import (
	"context"
	"io"
	"time"

	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema"
	"github.com/gravitational/teleport/lib/session"
)

const (
	// perCommandTimeout bounds each individual command-analysis inference call (SummarizeCommand). The command-analysis
	// path fans out one call per command (and per command-output chunk), so each call is bounded independently rather
	// than sharing a single job-wide deadline.
	perCommandTimeout = 5 * time.Minute
	// perChunkTimeout bounds each chunk/synthesis/simple inference call: SummarizeMultipleCommands (text chunk + final
	// synthesis),
	// SummarizeMultipleImages (per desktop image chunk), SummarizeDesktopSession (desktop final synthesis) and the simple
	// Summarize call.
	perChunkTimeout = 10 * time.Minute
	// maxSummarizationTimeout is the overall ceiling for a single summarization job, applied in summarizeNow once a
	// concurrency slot is held. Per-call timeouts let a large session run as long as it legitimately needs; this is only
	// a backstop against a pathological runaway. A job that hits it is failed (terminal ERROR) rather than left pending.
	maxSummarizationTimeout = 2 * time.Hour
	// defaultSlotAcquireTimeout bounds how long summarizeNow waits for the concurrency slots before giving up, separately
	// from maxSummarizationTimeout, so time spent queueing for a slot under load doesn't eat into the inference budget.
	// It's generous because there is no automatic retry: a transient slot shortage should rarely fail a summary outright.
	defaultSlotAcquireTimeout = 30 * time.Minute
	// summaryUploadTimeout bounds the final persist of the terminal summary, separately from the inference budget, so a
	// job that exhausted maxSummarizationTimeout still records its terminal state instead of failing to upload.
	summaryUploadTimeout = time.Minute

	accessGraphPushTimeout = 10 * time.Minute

	eventEmitTimeout = time.Minute
)

// timeoutProvider wraps an [InferenceProvider], bounding every inference call with a per-call timeout so a single slow
// or hung model call can't stall the whole summarization.
// The overall job is separately bounded by maxSummarizationTimeout in summarizeNow. Methods that don't make inference
// calls are inherited unchanged from the embedded provider.
type timeoutProvider struct {
	InferenceProvider
	commandTimeout time.Duration
	chunkTimeout   time.Duration
}

// newTimeoutProvider wraps p so each inference call is bounded by its per-unit timeout.
func newTimeoutProvider(p InferenceProvider) *timeoutProvider {
	return &timeoutProvider{
		InferenceProvider: p,
		commandTimeout:    perCommandTimeout,
		chunkTimeout:      perChunkTimeout,
	}
}

func (t *timeoutProvider) Summarize(ctx context.Context, sessionID session.ID, systemPrompt string, reader io.ReadCloser) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, t.chunkTimeout)
	defer cancel()

	return t.InferenceProvider.Summarize(ctx, sessionID, systemPrompt, reader)
}

func (t *timeoutProvider) SummarizeCommand(ctx context.Context, sessionID session.ID, username, loginName, command string) (*schema.CommandAnalysis, error) {
	ctx, cancel := context.WithTimeout(ctx, t.commandTimeout)
	defer cancel()

	return t.InferenceProvider.SummarizeCommand(ctx, sessionID, username, loginName, command)
}

func (t *timeoutProvider) SummarizeMultipleCommands(ctx context.Context, sessionID session.ID, username, loginName, prompt string) (*schema.SessionAnalysis, error) {
	ctx, cancel := context.WithTimeout(ctx, t.chunkTimeout)
	defer cancel()

	return t.InferenceProvider.SummarizeMultipleCommands(ctx, sessionID, username, loginName, prompt)
}

func (t *timeoutProvider) SummarizeMultipleImages(ctx context.Context, sessionID session.ID, systemPrompt string, images []schema.ImageData) (*schema.DesktopScreenshotAnalysis, error) {
	ctx, cancel := context.WithTimeout(ctx, t.chunkTimeout)
	defer cancel()

	return t.InferenceProvider.SummarizeMultipleImages(ctx, sessionID, systemPrompt, images)
}

func (t *timeoutProvider) SummarizeDesktopSession(ctx context.Context, sessionID session.ID, systemPrompt, prompt string) (*schema.DesktopSessionAnalysis, error) {
	ctx, cancel := context.WithTimeout(ctx, t.chunkTimeout)
	defer cancel()

	return t.InferenceProvider.SummarizeDesktopSession(ctx, sessionID, systemPrompt, prompt)
}
