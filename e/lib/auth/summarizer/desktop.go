package summarizer

import (
	"context"
	"encoding/base64"
	"log/slog"
	"strings"
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

// Anthropic's published vision-token heuristic: scale to fit a 1568x1568 box, then divide pixel count by 750.
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
		result.SetState(summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS)

		return nil
	}

	if len(d.allEvents) == 0 {
		s.logger.DebugContext(ctx, "No notable events detected, skipping synthesis", "session_id", details.sessionID)
		result.SetState(summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS)

		return nil
	}

	d.allEvents = consolidateAdjacentEvents(d.allEvents)

	sessionAnalysis, err := d.synthesize(ctx)
	if err != nil {
		return handleError(ctx, s.logger, result, err, "Failed to synthesize desktop session analysis")
	}

	result.SetEnhancedSummary(schema.DesktopSessionAnalysisToProto(sessionAnalysis, d.allEvents))
	result.SetState(summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS)

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
// runs until eventsCh signals end-of-stream (closed), errCh delivers an error, or ctx is canceled, then flushes
// the processor's final pending screenshot. AuditLog.StreamSessionEvents closes eventsCh on clean EOF or sends on
// errCh on error, but never both and never closes errCh. The returned stats are unsafe to read until the calling
// goroutine signals termination (see the screenshotStreamStats doc).
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

readLoop:
	for {
		select {
		case <-ctx.Done():
			stats.err = trace.Wrap(ctx.Err())
			return stats

		case evt, ok := <-eventsCh:
			if !ok {
				// End-of-stream: the streamer closes eventsCh on clean EOF.
				break readLoop
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
				// errCh closed without a value (uncommon; not part of the streamer
				// contract but harmless). Keep reading from eventsCh.
				errCh = nil
				continue
			}
			if err != nil {
				stats.err = trace.Wrap(err)
				return stats
			}
		}
	}

	// AuditLog.StreamSessionEvents closes eventsCh on EOF without closing errCh,
	// so do one non-blocking read in case an error was already queued before
	// the EOF was observed.
	select {
	case err, ok := <-errCh:
		if ok && err != nil {
			stats.err = trace.Wrap(err)
			return stats
		}
	default:
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
	// surfaced as a placeholder event with the right timespan. chunkTimes is parallel to chunk and records each
	// screenshot's session-relative StartTime/EndTime so events can resolve their LLM-emitted screenshot indices
	// back into real durations.
	chunk       [][]byte
	chunkTimes  []chunkScreenshotTime
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

// chunkScreenshotTime records the session-relative timestamps of a single screenshot in the current chunk so the
// LLM-emitted StartScreenshotIndex / EndScreenshotIndex can be resolved to real durations server-side.
type chunkScreenshotTime struct {
	start, end time.Duration
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
	d.chunkTimes = append(d.chunkTimes, chunkScreenshotTime{start: s.StartTime, end: s.EndTime})
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

	d.resolveEventTimestamps(ctx, analysis.NotableSessionEvents)

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

// resolveEventTimestamps converts each event's LLM-emitted StartScreenshotIndex / EndScreenshotIndex into the
// session-relative StartTime / EndTime strings the rest of the pipeline consumes. Out-of-range indices are clamped
// to the chunk bounds, and start > end is swapped, so a misbehaving model can't corrupt the merge key or the
// proto duration.
func (d *desktopSessionSummarizer) resolveEventTimestamps(ctx context.Context, events []schema.DesktopSessionEvent) {
	n := len(d.chunkTimes)
	if n == 0 {
		// Defensive: a chunk with no screenshots shouldn't reach inference, but if it does fall back to the
		// chunk's tracked bounds so the events still have a usable timestamp.
		fallback := desktop.FormatTimestamp(d.chunkStart)
		for i := range events {
			events[i].StartTime = fallback
			events[i].EndTime = fallback
		}
		return
	}

	for i := range events {
		startIdx := clampIndex(events[i].StartScreenshotIndex, n)
		endIdx := clampIndex(events[i].EndScreenshotIndex, n)
		if endIdx < startIdx {
			startIdx, endIdx = endIdx, startIdx
		}
		if startIdx != events[i].StartScreenshotIndex || endIdx != events[i].EndScreenshotIndex {
			d.logger.WarnContext(ctx, "Clamped out-of-range screenshot index from LLM",
				"session_id", d.details.sessionID,
				"chunk", d.chunkIndex,
				"event_index", i,
				"raw_start_index", events[i].StartScreenshotIndex,
				"raw_end_index", events[i].EndScreenshotIndex,
				"chunk_screenshots", n,
			)
			events[i].StartScreenshotIndex = startIdx
			events[i].EndScreenshotIndex = endIdx
		}

		events[i].StartTime = desktop.FormatTimestamp(d.chunkTimes[startIdx].start)
		events[i].EndTime = desktop.FormatTimestamp(d.chunkTimes[endIdx].end)
	}
}

// clampIndex bounds an LLM-emitted index into [0, n) so a hallucinated value can't index out of range.
func clampIndex(i, n int) int {
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

// resetChunk drops the chunk's PNG references for GC and rewinds the slice for reuse.
func (d *desktopSessionSummarizer) resetChunk() {
	clear(d.chunk)

	d.chunk = d.chunk[:0]
	d.chunkTimes = d.chunkTimes[:0]
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

// mergeEvents folds incoming events into allEvents. Same-StartTime events from a prior chunk are replaced in place
// (the screenshots prompt instructs the LLM to keep the original StartTime when refining a continuing event), but
// multiple events from the current chunk sharing a StartTime are all preserved: intra-chunk uniqueness is the
// LLM's responsibility, and the merge step must not silently drop them. New StartTimes are appended up to
// maxSessionEvents.
func (d *desktopSessionSummarizer) mergeEvents(ctx context.Context, incoming []schema.DesktopSessionEvent) {
	if len(incoming) == 0 {
		return
	}

	if d.eventIndex == nil {
		d.eventIndex = make(map[string]int, len(incoming))
	}

	// Continuation lookups only resolve against events from prior chunks. Events appended during this call are
	// indexed after the loop so a later event in the same incoming batch can't overwrite an earlier one.
	priorCount := len(d.allEvents)

	dropped := 0
	for _, e := range incoming {
		if i, ok := d.eventIndex[e.StartTime]; ok && i < priorCount {
			d.allEvents[i] = e
			continue
		}

		if len(d.allEvents) >= maxSessionEvents {
			dropped++
			continue
		}

		d.allEvents = append(d.allEvents, e)
	}

	// Refresh the index from this chunk's appended events so the next chunk can detect StartTime continuations.
	// When intra-chunk duplicates exist, the most recent position wins, so a continuation only replaces the
	// last occurrence.
	for i := priorCount; i < len(d.allEvents); i++ {
		d.eventIndex[d.allEvents[i].StartTime] = i
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

// titleSimilarityThreshold is the minimum Jaccard overlap on significant words for two adjacent events with the
// same Category to be considered "the same activity" and folded together by consolidateAdjacentEvents. Tuned to
// catch obvious near-duplicates ("Watching fireplace video on YouTube" / "Watching fireplace video with pre-roll
// ad") without collapsing genuinely-distinct events that happen to share a word or two.
const titleSimilarityThreshold = 0.5

// titleStopwords are dropped before computing similarity. Small fixed set covering the most common low-signal
// English words; the list intentionally does not try to stem ("watching"/"watched" stay distinct), since
// stemming adds dependencies and the Jaccard threshold tolerates a single non-stem mismatch.
var titleStopwords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "and": {}, "or": {}, "with": {}, "on": {}, "in": {}, "to": {}, "of": {},
	"for": {}, "by": {}, "at": {}, "is": {}, "was": {}, "from": {}, "as": {}, "into": {}, "via": {}, "this": {},
	"that": {}, "it": {}, "its": {},
}

// consolidateAdjacentEvents folds adjacent events that describe the same activity into one event spanning the
// combined range. Two events are merged when they share the same Category, neither carries an inference-error
// placeholder, and the incoming event's TimelineTitle has >= titleSimilarityThreshold Jaccard overlap with the
// running union of significant words from the cluster being built. Comparing against the union (not just the most
// recent title) keeps a long chain stable when a "longest title wins" replacement drops a key anchor word. The
// earlier event keeps its StartTime / StartScreenshotIndex; the later's EndTime / EndScreenshotIndex extends it.
// Indicator slices are union-ed and risk fields take the higher value, so a flagged sub-frame can never disappear
// into a benign neighbor.
func consolidateAdjacentEvents(events []schema.DesktopSessionEvent) []schema.DesktopSessionEvent {
	if len(events) < 2 {
		return events
	}

	out := make([]schema.DesktopSessionEvent, 0, len(events))
	out = append(out, events[0])
	clusterWords := significantWords(events[0].TimelineTitle)

	for i := 1; i < len(events); i++ {
		prev := &out[len(out)-1]
		curr := events[i]
		currWords := significantWords(curr.TimelineTitle)
		if canMergeEvents(*prev, curr, clusterWords, currWords) {
			mergeIntoEvent(prev, curr)
			for w := range currWords {
				clusterWords[w] = struct{}{}
			}
			continue
		}
		out = append(out, curr)
		clusterWords = currWords
	}
	return out
}

func canMergeEvents(prev, curr schema.DesktopSessionEvent, prevWords, currWords map[string]struct{}) bool {
	if prev.Category == "" || prev.Category != curr.Category {
		return false
	}
	if prev.InferenceErrorMessage != "" || curr.InferenceErrorMessage != "" {
		return false
	}
	return wordSetSimilarity(prevWords, currWords) >= titleSimilarityThreshold
}

// mergeIntoEvent folds curr into prev. prev keeps its Start* (earlier), takes curr's End* (later), keeps the
// longer-looking TimelineTitle, unions all indicator slices, ORs the bool flags, and takes the higher of the
// two risk levels and scores so a flagged sub-frame is never lost.
func mergeIntoEvent(prev *schema.DesktopSessionEvent, curr schema.DesktopSessionEvent) {
	prev.EndTime = curr.EndTime
	prev.EndScreenshotIndex = curr.EndScreenshotIndex

	if len(curr.TimelineTitle) > len(prev.TimelineTitle) {
		prev.TimelineTitle = curr.TimelineTitle
	}
	if isHigherRisk(curr.RiskLevel, prev.RiskLevel) {
		prev.RiskLevel = curr.RiskLevel
	}
	if curr.RiskScore > prev.RiskScore {
		prev.RiskScore = curr.RiskScore
	}
	if prev.ThreatCategory == "none" && curr.ThreatCategory != "none" && curr.ThreatCategory != "" {
		prev.ThreatCategory = curr.ThreatCategory
	}

	prev.SuspiciousFlags = unionStrings(prev.SuspiciousFlags, curr.SuspiciousFlags)
	prev.SensitiveItems = unionStrings(prev.SensitiveItems, curr.SensitiveItems)
	prev.SuspiciousPatterns = unionStrings(prev.SuspiciousPatterns, curr.SuspiciousPatterns)
	prev.IOCs = unionStrings(prev.IOCs, curr.IOCs)
	prev.MitreAttackIDs = unionStrings(prev.MitreAttackIDs, curr.MitreAttackIDs)
	prev.Applications = unionStrings(prev.Applications, curr.Applications)
	prev.VisibleURLs = unionStrings(prev.VisibleURLs, curr.VisibleURLs)
	prev.VisibleFilePaths = unionStrings(prev.VisibleFilePaths, curr.VisibleFilePaths)

	prev.HasSensitiveData = prev.HasSensitiveData || curr.HasSensitiveData
	prev.PrivilegeEscalation = prev.PrivilegeEscalation || curr.PrivilegeEscalation
	prev.DataExfiltration = prev.DataExfiltration || curr.DataExfiltration
	prev.Persistence = prev.Persistence || curr.Persistence
}

// titleSimilarity returns the Jaccard overlap of significant (non-stopword) lowercased word tokens between two
// timeline titles. Empty either side returns 0. Used by tests and as a convenience wrapper around wordSetSimilarity.
func titleSimilarity(a, b string) float64 {
	return wordSetSimilarity(significantWords(a), significantWords(b))
}

// wordSetSimilarity returns the Jaccard index of two word sets. Empty either side returns 0.
func wordSetSimilarity(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	intersection := 0
	for w := range a {
		if _, ok := b[w]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// significantWords lowercases s, splits on whitespace and most punctuation, drops empty tokens, stopwords, and
// leading/trailing hyphens. Hyphens inside a token are preserved so compound concepts like "pre-roll" or
// "--no-sandbox" stay as a single significant word.
func significantWords(s string) map[string]struct{} {
	if s == "" {
		return nil
	}
	out := make(map[string]struct{})
	var sb strings.Builder
	flush := func() {
		if sb.Len() == 0 {
			return
		}
		w := strings.Trim(sb.String(), "-")
		sb.Reset()
		if w == "" {
			return
		}
		if _, isStop := titleStopwords[w]; isStop {
			return
		}
		out[w] = struct{}{}
	}
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			sb.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

func unionStrings(a, b []string) []string {
	if len(b) == 0 {
		return a
	}
	if len(a) == 0 {
		return append([]string(nil), b...)
	}
	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, s := range a {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, s := range b {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
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
