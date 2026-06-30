package summarizer

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema"
	"github.com/gravitational/teleport/lib/session"
)

// TestTimeoutProvider_PerCallDeadlines asserts that each inference call made through the timeout-wrapping provider
// receives a context bounded by the per-unit timeout for that call: 5 minutes per command, 10 minutes for every
// chunk/synthesis/simple call.
func TestTimeoutProvider_PerCallDeadlines(t *testing.T) {
	rec := newDeadlineRecordingProvider()
	p := newTimeoutProvider(rec)
	ctx := t.Context()

	_, _ = p.SummarizeCommand(ctx, "s", "u", "l", "cmd")
	_, _ = p.SummarizeMultipleCommands(ctx, "s", "u", "l", "prompt")
	_, _ = p.SummarizeMultipleImages(ctx, "s", "prompt", nil)
	_, _ = p.SummarizeDesktopSession(ctx, "s", "sys", "prompt")
	_, _ = p.Summarize(ctx, "s", "sys", io.NopCloser(strings.NewReader("")))

	assertBudget(t, rec, "SummarizeCommand", perCommandTimeout)
	assertBudget(t, rec, "SummarizeMultipleCommands", perChunkTimeout)
	assertBudget(t, rec, "SummarizeMultipleImages", perChunkTimeout)
	assertBudget(t, rec, "SummarizeDesktopSession", perChunkTimeout)
	assertBudget(t, rec, "Summarize", perChunkTimeout)
}

func assertBudget(t *testing.T, rec *deadlineRecordingProvider, method string, want time.Duration) {
	t.Helper()

	budget, ok := rec.budget(method)
	require.True(t, ok, "%s should receive a context with a deadline", method)
	require.InDelta(t, want, budget, float64(30*time.Second), "%s deadline budget", method)
}

// deadlineRecordingProvider records the remaining context budget observed at each inference call, so tests can assert
// that per-call timeouts are applied.
type deadlineRecordingProvider struct {
	mu      sync.Mutex
	budgets map[string]time.Duration
	hasDL   map[string]bool
}

func newDeadlineRecordingProvider() *deadlineRecordingProvider {
	return &deadlineRecordingProvider{
		budgets: make(map[string]time.Duration),
		hasDL:   make(map[string]bool),
	}
}

func (p *deadlineRecordingProvider) record(method string, ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()

	dl, ok := ctx.Deadline()
	p.hasDL[method] = ok

	if ok {
		p.budgets[method] = time.Until(dl)
	}
}

func (p *deadlineRecordingProvider) budget(method string) (time.Duration, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.budgets[method], p.hasDL[method]
}

func (p *deadlineRecordingProvider) Summarize(ctx context.Context, _ session.ID, _ string, r io.ReadCloser) (string, error) {
	p.record("Summarize", ctx)
	if r != nil {
		_ = r.Close()
	}

	return "", nil
}

func (p *deadlineRecordingProvider) SummarizeCommand(ctx context.Context, _ session.ID, _, _, _ string) (*schema.CommandAnalysis, error) {
	p.record("SummarizeCommand", ctx)

	return &schema.CommandAnalysis{}, nil
}

func (p *deadlineRecordingProvider) SummarizeMultipleCommands(ctx context.Context, _ session.ID, _, _, _ string) (*schema.SessionAnalysis, error) {
	p.record("SummarizeMultipleCommands", ctx)

	return &schema.SessionAnalysis{}, nil
}

func (p *deadlineRecordingProvider) SummarizeMultipleImages(ctx context.Context, _ session.ID, _ string, _ []schema.ImageData) (*schema.DesktopScreenshotAnalysis, error) {
	p.record("SummarizeMultipleImages", ctx)

	return &schema.DesktopScreenshotAnalysis{}, nil
}

func (p *deadlineRecordingProvider) SummarizeDesktopSession(ctx context.Context, _ session.ID, _, _ string) (*schema.DesktopSessionAnalysis, error) {
	p.record("SummarizeDesktopSession", ctx)

	return &schema.DesktopSessionAnalysis{}, nil
}

func (p *deadlineRecordingProvider) GetTotalTokens() (uint64, uint64) { return 0, 0 }
func (p *deadlineRecordingProvider) GetType() string                  { return "test" }
func (p *deadlineRecordingProvider) CondenseForEmbedding(context.Context, *summarizerv1pb.Summary) (string, error) {
	return "", nil
}

// TestSummarizeSimple_ReaderObservesPerCallTimeout asserts that the session reader created in summarizeSimple is bounded
// by the per-chunk budget rather than the job ceiling.
// The provider drains the reader before issuing the model request, so a stalled StreamSessionEvents must be interrupted
// by the per-call timeout instead of holding a concurrency slot until maxSummarizationTimeout.
func TestSummarizeSimple_ReaderObservesPerCallTimeout(t *testing.T) {
	streamer := &capturingStreamer{}
	s := &SessionSummarizer{streamer: streamer}

	details := &sessionDetails{
		sessionID:  "11111111-1111-1111-1111-111111111111",
		kind:       types.DatabaseSessionKind,
		provider:   newTimeoutProvider(newDeadlineRecordingProvider()),
		sessionEnd: &apievents.DatabaseSessionEnd{},
		now:        time.Now(),
	}

	// Mirror summarizeNow: the inference phase runs under the maxSummarizationTimeout job ceiling.
	ctx, cancel := context.WithTimeout(context.Background(), maxSummarizationTimeout)
	defer cancel()

	result := summarizerv1pb.Summary_builder{}.Build()
	require.NoError(t, s.summarizeSimple(ctx, slog.New(slog.DiscardHandler), result, details))

	budget, ok := streamer.budget()
	require.True(t, ok, "session reader stream context should carry a deadline")
	require.InDelta(t, perChunkTimeout, budget, float64(30*time.Second),
		"session reader must observe the per-call chunk budget, not the job ceiling")
}

// capturingStreamer records the context handed to StreamSessionEvents so a test can assert the deadline the session
// reader's drain observes.
type capturingStreamer struct {
	mu    sync.Mutex
	dl    time.Time
	hasDL bool
}

func (c *capturingStreamer) StreamSessionEvents(ctx context.Context, _ session.ID, _ int64) (chan apievents.AuditEvent, chan error) {
	c.mu.Lock()
	c.dl, c.hasDL = ctx.Deadline()
	c.mu.Unlock()

	eventsCh := make(chan apievents.AuditEvent)
	close(eventsCh)

	return eventsCh, make(chan error, 1)
}

func (c *capturingStreamer) budget() (time.Duration, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.hasDL {
		return 0, false
	}

	return time.Until(c.dl), true
}
