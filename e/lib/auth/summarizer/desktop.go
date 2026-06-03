package summarizer

import (
	"context"
	"encoding/base64"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/desktop"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema"
)

// Per-chunk limits for screenshot batches sent to AI inference. Bounded so a single batch fits within Claude's
// vision constraints (max 1568px dimension, ~1600 tokens per ~1.15MP image, 200K context window) and provider
// per-request payload limits.
const (
	desktopMaxImagesPerChunk      = 20
	desktopMaxImageTokensPerChunk = 50_000
	desktopMaxImageBytesPerChunk  = 5 * 1024 * 1024
	// Buffered so the streamer can produce screenshots while the previous chunk's inference is in flight.
	//nolint:unused // consumed by summarizeDesktopSession, which is wired into SessionSummarizer dispatch in the ryan/wire-desktop-summarization stack layer
	screenshotChannelBuffer = 8
	// Bounds the per-session accumulator. Without this, a pathological LLM output (many distinct StartTime values
	// that bypass dedup) could grow allEvents without limit.
	maxSessionEvents = 2000
	// Conservative cap on the user-prompt portion of the synthesis request. Roughly 60K tokens, well below the 200K
	// context window, leaving headroom for the system prompt and response.
	desktopMaxSynthesisPromptBytes = 250 * 1024
)

// Anthropic's published vision-token heuristic: scale to fit a 1568×1568 box, then divide pixel count by 750.
const (
	claudeMaxImageDimension   = 1568
	claudeImagePixelsPerToken = 750
)

// screenshotStreamStats is shared between the streaming goroutine (writer) and the main goroutine (reader). Reads
// are valid only after the goroutine signals completion via close(streamDone); the close establishes the
// happens-before edge for the writes.
type screenshotStreamStats struct {
	eventCount       int
	lastEventTime    time.Time
	totalScreenshots int
	totalBytes       int
	err              error
}

// summarizeDesktopSession streams events from a desktop recording, captures screenshots, batches them for chunked
// AI analysis, and runs a final session-level synthesis pass over the events extracted from those chunks.
//
//nolint:unused // entry point wired into SessionSummarizer dispatch in the ryan/wire-desktop-summarization stack layer
func (s *SessionSummarizer) summarizeDesktopSession(
	ctx context.Context,
	result *summarizerv1pb.Summary,
	details sessionDetails,
) error {
	s.logger.DebugContext(ctx, "Starting desktop session summarization", "session_id", details.sessionID)
	summarizeStart := time.Now()

	processor := desktop.NewRecordingProcessor(s.glyphCache())
	defer processor.Release()

	streamCtx, cancelStream := context.WithCancel(ctx)
	eventsCh, errCh := s.streamer.StreamSessionEvents(streamCtx, details.sessionID, 0)

	screenshotCh := make(chan desktop.ScreenshotResult, screenshotChannelBuffer)
	streamDone := make(chan struct{})
	var stats screenshotStreamStats

	go func() {
		defer close(streamDone)
		defer close(screenshotCh)

		stats = streamScreenshots(streamCtx, processor, eventsCh, errCh, screenshotCh)
	}()

	// processor.Release() (deferred above) must not run while the streamer goroutine is still using it. cancelStream
	// unblocks the goroutine on early returns so the wait can't deadlock.
	defer func() {
		cancelStream()
		<-streamDone
	}()

	d := &desktopSessionSummarizer{
		logger:  s.logger,
		details: details,
		chunk:   make([][]byte, 0, desktopMaxImagesPerChunk),
	}

	if err := d.summarize(ctx, screenshotCh); err != nil {
		return handleError(ctx, s.logger, result, err, "Failed to summarize desktop screenshots")
	}

	// Block until the streamer signals completion before reading any stats field; see the type doc for the
	// happens-before contract.
	<-streamDone
	if stats.err != nil {
		return handleError(ctx, s.logger, result, stats.err, "Failed to stream desktop session events")
	}

	if stats.totalScreenshots == 0 {
		s.logger.DebugContext(ctx, "No screenshots captured, skipping summarization", "session_id", details.sessionID)
		result.State = summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS

		return nil
	}

	if len(d.allEvents) == 0 {
		s.logger.DebugContext(ctx, "No notable events detected, skipping synthesis", "session_id", details.sessionID)
		result.State = summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS

		return nil
	}

	sessionAnalysis, err := d.synthesize(ctx)
	if err != nil {
		return handleError(ctx, s.logger, result, err, "Failed to synthesize desktop session analysis")
	}

	// TODO(ryan): in the next PR, convert sessionAnalysis + d.allEvents into Summary proto fields.
	_ = sessionAnalysis

	result.State = summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS

	var sessionDuration time.Duration
	if !processor.StartTime().IsZero() && !stats.lastEventTime.IsZero() {
		sessionDuration = stats.lastEventTime.Sub(processor.StartTime())
	}

	s.logger.InfoContext(ctx, "Desktop session summarization complete",
		"session_id", details.sessionID,
		"session_duration", sessionDuration,
		"processing_time", time.Since(summarizeStart),
		"total_events", stats.eventCount,
		"screenshots", stats.totalScreenshots,
		"screenshot_bytes_total", stats.totalBytes,
		"image_tokens_total", d.totalImageTokens,
		"chunks_processed", d.chunkIndex,
	)

	return nil
}

type screenshotProcessor interface {
	ProcessEvent(evt apievents.AuditEvent) (*desktop.ScreenshotResult, error)
	Flush() ([]desktop.ScreenshotResult, error)
}

// streamScreenshots drives processor with events from eventsCh and forwards any captured screenshots to out. It
// runs until both eventsCh and errCh are drained or ctx is canceled, then flushes the processor's final pending
// screenshot. The returned stats are unsafe to read until the calling goroutine signals termination — see the
// screenshotStreamStats doc.
func streamScreenshots(
	ctx context.Context,
	processor screenshotProcessor,
	eventsCh <-chan apievents.AuditEvent,
	errCh <-chan error,
	out chan<- desktop.ScreenshotResult,
) screenshotStreamStats {
	var stats screenshotStreamStats

	send := func(res desktop.ScreenshotResult) bool {
		stats.totalScreenshots++
		stats.totalBytes += len(res.PNG)

		select {
		case out <- res:
			return true
		case <-ctx.Done():
			return false
		}
	}

	// Both channels must be observed closed before exit; if errCh closes first, eventsCh might still deliver
	// real events (and vice versa).
	for eventsCh != nil || errCh != nil {
		select {
		case <-ctx.Done():
			stats.err = trace.Wrap(ctx.Err())
			return stats

		case evt, ok := <-eventsCh:
			if !ok {
				eventsCh = nil
				continue
			}

			stats.eventCount++
			stats.lastEventTime = evt.GetTime()

			res, err := processor.ProcessEvent(evt)
			if err != nil {
				stats.err = trace.Wrap(err)
				return stats
			}

			if res != nil && !send(*res) {
				return stats
			}

		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if err != nil {
				stats.err = trace.Wrap(err)
				return stats
			}
		}
	}

	results, err := processor.Flush()
	if err != nil {
		stats.err = trace.Wrap(err)

		return stats
	}

	for _, res := range results {
		if !send(res) {
			return stats
		}
	}

	return stats
}

// desktopSessionSummarizer holds the per-session state for chunk batching, per-chunk inference, and final
// synthesis. Single-goroutine; the streamer handles concurrency.
type desktopSessionSummarizer struct {
	logger  *slog.Logger
	details sessionDetails

	// chunk is the in-flight batch of PNG bytes. Capacity is pre-allocated and the slice is reused across flushes
	// to release prior PNGs for GC. chunkStart/End track the session-relative time range covered so a failed chunk can be
	// surfaced as a placeholder event with the right timespan.
	chunk       [][]byte
	chunkTokens int
	chunkBytes  int
	chunkIndex  int
	chunkStart  time.Duration
	chunkEnd    time.Duration

	// eventIndex maps StartTime to position in allEvents, kept across chunks so merge stays O(1) per event instead
	// of rebuilding the map on every flush.
	allEvents    []schema.DesktopSessionEvent
	eventIndex   map[string]int
	prevAnalysis *schema.DesktopScreenshotAnalysis

	// Partial-failure flags propagated to DesktopSessionAnalysis.TooLarge / .ScreenshotAnalysisFailed; the proto
	// layer maps either to NeedsFurtherReview on the surfaced summary.
	tooLarge                 bool
	screenshotAnalysisFailed bool

	totalImageTokens int
}

//nolint:unused // called only by summarizeDesktopSession, which is wired into SessionSummarizer dispatch in the ryan/wire-desktop-summarization stack layer
func (d *desktopSessionSummarizer) summarize(ctx context.Context, screenshotCh <-chan desktop.ScreenshotResult) error {
	for s := range screenshotCh {
		if err := d.accumulate(ctx, s); err != nil {
			return err
		}
	}

	return d.flushChunk(ctx)
}

func (d *desktopSessionSummarizer) accumulate(ctx context.Context, s desktop.ScreenshotResult) error {
	tokens := countClaudeImageTokens(s.Width, s.Height)
	d.totalImageTokens += tokens

	// Providers transmit images base64-encoded, so the byte budget must account for the encoded size, not the raw
	// PNG length.
	b64Size := base64.StdEncoding.EncodedLen(len(s.PNG))

	if len(d.chunk) > 0 &&
		(d.chunkTokens+tokens > desktopMaxImageTokensPerChunk ||
			len(d.chunk) >= desktopMaxImagesPerChunk ||
			d.chunkBytes+b64Size > desktopMaxImageBytesPerChunk) {
		if err := d.flushChunk(ctx); err != nil {
			return err
		}
	}

	if tokens > desktopMaxImageTokensPerChunk || b64Size > desktopMaxImageBytesPerChunk {
		d.recordOversizeScreenshot(ctx, s, tokens, b64Size)
		return nil
	}

	if len(d.chunk) == 0 {
		d.chunkStart = s.StartTime
	}
	d.chunkEnd = s.EndTime

	d.chunk = append(d.chunk, s.PNG)
	d.chunkTokens += tokens
	d.chunkBytes += b64Size

	return nil
}

func (d *desktopSessionSummarizer) recordOversizeScreenshot(ctx context.Context, s desktop.ScreenshotResult, tokens, b64Size int) {
	d.screenshotAnalysisFailed = true

	d.logger.WarnContext(ctx, "Skipping desktop screenshot that exceeds per-chunk caps",
		"session_id", d.details.sessionID,
		"tokens", tokens,
		"b64_bytes", b64Size,
		"max_tokens", desktopMaxImageTokensPerChunk,
		"max_bytes", desktopMaxImageBytesPerChunk,
		"start_time", s.StartTime,
		"end_time", s.EndTime,
	)

	d.mergeEvents(ctx, []schema.DesktopSessionEvent{{
		Category:              "other",
		StartTime:             desktop.FormatTimestamp(s.StartTime),
		EndTime:               desktop.FormatTimestamp(s.EndTime),
		RiskLevel:             "high",
		ThreatCategory:        "none",
		TimelineTitle:         "Screenshot too large",
		TimelineSubtitle:      "Skipped",
		ShortDescription:      "Screenshot exceeded inference size limits and was not analyzed.",
		DetailedDescription:   "A screenshot in this window exceeded the per-chunk size or token cap and was skipped before inference.",
		InferenceErrorMessage: "screenshot exceeds per-chunk size caps",
	}})
}

// flushChunk sends the current chunk for inference and merges the returned events. A provider error is converted
// into a placeholder event covering the chunk's timespan so the rest of the recording can still be processed; only
// a canceled context aborts the run.
func (d *desktopSessionSummarizer) flushChunk(ctx context.Context) error {
	if len(d.chunk) == 0 {
		return nil
	}

	defer d.resetChunk()

	d.chunkIndex++
	d.logger.DebugContext(ctx, "Processing desktop screenshot chunk",
		"session_id", d.details.sessionID,
		"chunk", d.chunkIndex,
		"screenshots", len(d.chunk),
		"tokens", d.chunkTokens,
	)

	images := make([]schema.ImageData, len(d.chunk))
	for i, png := range d.chunk {
		images[i] = schema.ImageData{Data: png}
	}

	analysis, err := d.details.provider.SummarizeMultipleImages(
		ctx,
		d.details.sessionID,
		desktop.BuildScreenshotsSystemPrompt(d.prevAnalysis),
		images,
	)
	if err != nil {
		// A canceled ctx means the whole run is aborting; don't synthesize a placeholder for a chunk that won't
		// be reported.
		if ctx.Err() != nil {
			return trace.Wrap(err)
		}

		d.recordChunkFailure(ctx, err)
		return nil
	}

	d.mergeEvents(ctx, analysis.NotableSessionEvents)
	d.prevAnalysis = analysis

	d.logger.DebugContext(ctx, "Chunk processed",
		"session_id", d.details.sessionID,
		"chunk", d.chunkIndex,
		"events_from_chunk", len(analysis.NotableSessionEvents),
		"total_events", len(d.allEvents),
	)

	return nil
}

// resetChunk drops the chunk's PNG references for GC and rewinds the slice for reuse.
func (d *desktopSessionSummarizer) resetChunk() {
	clear(d.chunk)

	d.chunk = d.chunk[:0]
	d.chunkTokens = 0
	d.chunkBytes = 0
	d.chunkStart = 0
	d.chunkEnd = 0
}

func (d *desktopSessionSummarizer) recordChunkFailure(ctx context.Context, err error) {
	d.screenshotAnalysisFailed = true

	d.logger.WarnContext(ctx, "Desktop screenshot chunk inference failed; continuing with remaining chunks",
		"session_id", d.details.sessionID,
		"chunk", d.chunkIndex,
		"chunk_start", d.chunkStart,
		"chunk_end", d.chunkEnd,
		"error", err,
	)

	userMessage := trace.UserMessage(err)
	// Route through mergeEvents so the maxSessionEvents cap applies to failure placeholders too, and so a retried
	// chunk with the same StartTime replaces rather than duplicates.
	d.mergeEvents(ctx, []schema.DesktopSessionEvent{{
		Category:              "other",
		StartTime:             desktop.FormatTimestamp(d.chunkStart),
		EndTime:               desktop.FormatTimestamp(d.chunkEnd),
		RiskLevel:             "high",
		ThreatCategory:        "none",
		TimelineTitle:         "Screenshot analysis unavailable",
		TimelineSubtitle:      "Inference failed",
		ShortDescription:      "Screenshot analysis failed for this chunk; activity in this window was not analyzed.",
		DetailedDescription:   "The inference provider failed to analyze the screenshots in this chunk: " + userMessage,
		InferenceErrorMessage: userMessage,
	}})
}

// mergeEvents folds incoming events into allEvents. Same-StartTime events replace the existing entry (the
// screenshots prompt instructs the LLM to keep the original StartTime when refining a continuing event); new
// StartTimes are appended up to maxSessionEvents.
func (d *desktopSessionSummarizer) mergeEvents(ctx context.Context, incoming []schema.DesktopSessionEvent) {
	if len(incoming) == 0 {
		return
	}

	if d.eventIndex == nil {
		d.eventIndex = make(map[string]int, len(incoming))
	}

	dropped := 0
	for _, e := range incoming {
		if i, ok := d.eventIndex[e.StartTime]; ok {
			d.allEvents[i] = e
			continue
		}

		if len(d.allEvents) >= maxSessionEvents {
			dropped++
			continue
		}

		d.eventIndex[e.StartTime] = len(d.allEvents)
		d.allEvents = append(d.allEvents, e)
	}

	if dropped > 0 {
		d.tooLarge = true
		d.logger.WarnContext(ctx, "Dropping desktop session events past per-session cap",
			"session_id", d.details.sessionID,
			"dropped", dropped,
			"cap", maxSessionEvents,
		)
	}
}

func (d *desktopSessionSummarizer) synthesize(ctx context.Context) (*schema.DesktopSessionAnalysis, error) {
	d.logger.DebugContext(ctx, "Synthesizing desktop session analysis",
		"session_id", d.details.sessionID,
		"total_events", len(d.allEvents),
		"chunks_processed", d.chunkIndex,
	)

	events := d.allEvents
	systemPrompt, prompt := desktop.BuildSessionSynthesisPrompt(events)
	for len(prompt) > desktopMaxSynthesisPromptBytes && len(events) > 1 {
		drop := max(1, len(events)/10)
		events = events[drop:]
		_, prompt = desktop.BuildSessionSynthesisPrompt(events)
	}
	if len(events) < len(d.allEvents) {
		d.tooLarge = true
		d.logger.WarnContext(ctx, "Trimmed earliest desktop session events to fit synthesis prompt budget",
			"session_id", d.details.sessionID,
			"kept", len(events),
			"dropped", len(d.allEvents)-len(events),
			"prompt_bytes", len(prompt),
			"budget_bytes", desktopMaxSynthesisPromptBytes,
		)
	}

	analysis, err := d.details.provider.SummarizeDesktopSession(ctx, d.details.sessionID, systemPrompt, prompt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Floor risk to medium on partial failure: the LLM only saw a subset of the session and may understate the
	// real severity.
	analysis.TooLarge = d.tooLarge
	analysis.ScreenshotAnalysisFailed = d.screenshotAnalysisFailed
	if (d.tooLarge || d.screenshotAnalysisFailed) && !isHigherRisk(analysis.RiskLevel, "medium") {
		analysis.RiskLevel = "medium"
	}

	return analysis, nil
}

// countClaudeImageTokens estimates Claude vision tokens for an image of the given dimensions.
func countClaudeImageTokens(width, height int) int {
	w, h := width, height
	if w > claudeMaxImageDimension || h > claudeMaxImageDimension {
		scale := float64(claudeMaxImageDimension) / float64(max(w, h))
		w = int(float64(w) * scale)
		h = int(float64(h) * scale)
	}

	return (w * h) / claudeImagePixelsPerToken
}
