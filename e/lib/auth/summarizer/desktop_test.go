package summarizer

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/desktop"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema"
	"github.com/gravitational/teleport/lib/session"
)

func TestDesktopSessionSummarizer_MergeEvents(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	makeEvent := func(start, title string) schema.DesktopSessionEvent {
		return schema.DesktopSessionEvent{
			StartTime:     start,
			TimelineTitle: title,
		}
	}

	tests := []struct {
		name           string
		existing       []schema.DesktopSessionEvent
		incoming       []schema.DesktopSessionEvent
		wantTitlesByID map[string]string
		wantOrder      []string // StartTime values, in order
	}{
		{
			name:           "empty incoming preserves existing",
			existing:       []schema.DesktopSessionEvent{makeEvent("0:00", "first")},
			incoming:       nil,
			wantTitlesByID: map[string]string{"0:00": "first"},
			wantOrder:      []string{"0:00"},
		},
		{
			name:     "appends new events",
			existing: []schema.DesktopSessionEvent{makeEvent("0:00", "first")},
			incoming: []schema.DesktopSessionEvent{makeEvent("0:10", "second")},
			wantTitlesByID: map[string]string{
				"0:00": "first",
				"0:10": "second",
			},
			wantOrder: []string{"0:00", "0:10"},
		},
		{
			name:     "replaces matching StartTime in place",
			existing: []schema.DesktopSessionEvent{makeEvent("0:00", "old")},
			incoming: []schema.DesktopSessionEvent{makeEvent("0:00", "new")},
			wantTitlesByID: map[string]string{
				"0:00": "new",
			},
			wantOrder: []string{"0:00"},
		},
		{
			name:     "mixed replace and append",
			existing: []schema.DesktopSessionEvent{makeEvent("0:00", "first"), makeEvent("0:10", "second")},
			incoming: []schema.DesktopSessionEvent{makeEvent("0:10", "second-refined"), makeEvent("0:20", "third")},
			wantTitlesByID: map[string]string{
				"0:00": "first",
				"0:10": "second-refined",
				"0:20": "third",
			},
			wantOrder: []string{"0:00", "0:10", "0:20"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := newTestSummarizer(t, &countingProvider{})
			d.allEvents = append([]schema.DesktopSessionEvent(nil), tt.existing...)
			for i, e := range d.allEvents {
				if d.eventIndex == nil {
					d.eventIndex = make(map[string]int, len(d.allEvents))
				}
				d.eventIndex[e.StartTime] = i
			}

			d.mergeEvents(ctx, tt.incoming)

			require.Len(t, d.allEvents, len(tt.wantOrder))
			for i, start := range tt.wantOrder {
				require.Equal(t, start, d.allEvents[i].StartTime, "position %d", i)
				require.Equal(t, tt.wantTitlesByID[start], d.allEvents[i].TimelineTitle, "position %d", i)
			}
		})
	}
}

func TestDesktopSessionSummarizer_MergeEventsCapSetsTooLarge(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	d := newTestSummarizer(t, &countingProvider{})

	incoming := make([]schema.DesktopSessionEvent, maxSessionEvents+50)
	for i := range incoming {
		incoming[i] = schema.DesktopSessionEvent{StartTime: time.Duration(i).String()}
	}

	d.mergeEvents(ctx, incoming)
	require.True(t, d.tooLarge)
	require.Len(t, d.allEvents, maxSessionEvents)

	// Repeated call with the same StartTimes still updates in place; cap is not exceeded.
	d.mergeEvents(ctx, incoming[:10])
	require.Len(t, d.allEvents, maxSessionEvents)
}

func TestDesktopSessionSummarizer_AccumulateFlushTriggers(t *testing.T) {
	t.Parallel()

	smallPNG := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}

	tests := []struct {
		name        string
		results     []desktop.ScreenshotResult
		wantFlushes int
	}{
		{
			name: "no flush below all limits",
			results: []desktop.ScreenshotResult{
				{PNG: smallPNG, Width: 100, Height: 100},
				{PNG: smallPNG, Width: 100, Height: 100},
			},
			wantFlushes: 0,
		},
		{
			name: "flush on image count limit",
			results: func() []desktop.ScreenshotResult {
				out := make([]desktop.ScreenshotResult, desktopMaxImagesPerChunk+1)
				for i := range out {
					out[i] = desktop.ScreenshotResult{PNG: smallPNG, Width: 100, Height: 100}
				}
				return out
			}(),
			wantFlushes: 1,
		},
		{
			// At max dim each screenshot is ~3279 tokens (1568*1568/750). 16 of them = 52,464 tokens, just past the
			// 50k limit; image count is still 16 (< 20) so the token branch wins.
			name: "flush on token limit",
			results: func() []desktop.ScreenshotResult {
				out := make([]desktop.ScreenshotResult, 16)
				for i := range out {
					out[i] = desktop.ScreenshotResult{PNG: smallPNG, Width: claudeMaxImageDimension, Height: claudeMaxImageDimension}
				}
				return out
			}(),
			wantFlushes: 1,
		},
		{
			name: "flush on byte limit",
			results: []desktop.ScreenshotResult{
				{PNG: make([]byte, desktopMaxImageBytesPerChunk*3/4), Width: 10, Height: 10},
				{PNG: make([]byte, desktopMaxImageBytesPerChunk*3/4), Width: 10, Height: 10},
			},
			wantFlushes: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider := &countingProvider{}
			d := newTestSummarizer(t, provider)

			for _, r := range tt.results {
				require.NoError(t, d.accumulate(t.Context(), r))
			}

			require.EqualValues(t, tt.wantFlushes, provider.imageCalls.Load())
		})
	}
}

func TestDesktopSessionSummarizer_AccumulatePlumbsChunkTimespan(t *testing.T) {
	t.Parallel()

	d := newTestSummarizer(t, &countingProvider{})

	results := []desktop.ScreenshotResult{
		{PNG: []byte{1}, Width: 10, Height: 10, StartTime: 1 * time.Second, EndTime: 1 * time.Second},
		{PNG: []byte{2}, Width: 10, Height: 10, StartTime: 4 * time.Second, EndTime: 6 * time.Second},
		{PNG: []byte{3}, Width: 10, Height: 10, StartTime: 9 * time.Second, EndTime: 11 * time.Second},
	}
	for _, r := range results {
		require.NoError(t, d.accumulate(t.Context(), r))
	}

	// chunkStart pins to the first screenshot in the chunk; chunkEnd advances to the most recent EndTime.
	require.Equal(t, 1*time.Second, d.chunkStart)
	require.Equal(t, 11*time.Second, d.chunkEnd)
}

func TestDesktopSessionSummarizer_FlushChunkEmpty(t *testing.T) {
	t.Parallel()

	provider := &countingProvider{}
	d := newTestSummarizer(t, provider)

	require.NoError(t, d.flushChunk(t.Context()))
	require.EqualValues(t, 0, provider.imageCalls.Load())
	require.Equal(t, 0, d.chunkIndex)
}

func TestDesktopSessionSummarizer_FlushChunkResetsState(t *testing.T) {
	t.Parallel()

	provider := &countingProvider{
		analysis: &schema.DesktopScreenshotAnalysis{
			NotableSessionEvents: []schema.DesktopSessionEvent{{TimelineTitle: "ev"}},
		},
	}
	d := newTestSummarizer(t, provider)
	d.chunk = append(d.chunk, []byte("png-bytes"))
	d.chunkTimes = append(d.chunkTimes, chunkScreenshotTime{start: 0, end: 0})
	d.chunkTokens = 100
	d.chunkBytes = 200

	require.NoError(t, d.flushChunk(t.Context()))

	require.EqualValues(t, 1, provider.imageCalls.Load())
	require.Equal(t, 1, d.chunkIndex)
	require.Empty(t, d.chunk)
	require.Empty(t, d.chunkTimes)
	require.Equal(t, 0, d.chunkTokens)
	require.Equal(t, 0, d.chunkBytes)
	require.Len(t, d.allEvents, 1)
	require.Equal(t, provider.analysis, d.prevAnalysis)
}

func TestDesktopSessionSummarizer_ResolveEventTimestamps(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	tests := []struct {
		name        string
		chunkTimes  []chunkScreenshotTime
		chunkStart  time.Duration
		events      []schema.DesktopSessionEvent
		wantStart   []string
		wantEnd     []string
		wantIndices [][2]int // expected post-clamp [start, end]
	}{
		{
			name: "in-range indices resolve to bar timestamps",
			chunkTimes: []chunkScreenshotTime{
				{start: 0, end: 0},
				{start: 5 * time.Second, end: 5 * time.Second},
				{start: 12 * time.Second, end: 30 * time.Second},
			},
			events: []schema.DesktopSessionEvent{
				{StartScreenshotIndex: 0, EndScreenshotIndex: 0},
				{StartScreenshotIndex: 1, EndScreenshotIndex: 2},
			},
			wantStart:   []string{"0:00", "0:05"},
			wantEnd:     []string{"0:00", "0:30"},
			wantIndices: [][2]int{{0, 0}, {1, 2}},
		},
		{
			name: "out-of-range indices clamp to chunk bounds",
			chunkTimes: []chunkScreenshotTime{
				{start: 2 * time.Second, end: 2 * time.Second},
				{start: 8 * time.Second, end: 8 * time.Second},
			},
			events: []schema.DesktopSessionEvent{
				{StartScreenshotIndex: -3, EndScreenshotIndex: 99},
			},
			wantStart:   []string{"0:02"},
			wantEnd:     []string{"0:08"},
			wantIndices: [][2]int{{0, 1}},
		},
		{
			name: "inverted indices swap",
			chunkTimes: []chunkScreenshotTime{
				{start: 1 * time.Second, end: 1 * time.Second},
				{start: 7 * time.Second, end: 7 * time.Second},
			},
			events: []schema.DesktopSessionEvent{
				{StartScreenshotIndex: 1, EndScreenshotIndex: 0},
			},
			wantStart:   []string{"0:01"},
			wantEnd:     []string{"0:07"},
			wantIndices: [][2]int{{0, 1}},
		},
		{
			name:       "empty chunkTimes falls back to chunkStart",
			chunkTimes: nil,
			chunkStart: 4 * time.Second,
			events: []schema.DesktopSessionEvent{
				{StartScreenshotIndex: 0, EndScreenshotIndex: 0},
			},
			wantStart:   []string{"0:04"},
			wantEnd:     []string{"0:04"},
			wantIndices: [][2]int{{0, 0}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := newTestSummarizer(t, &countingProvider{})
			d.chunkTimes = tt.chunkTimes
			d.chunkStart = tt.chunkStart

			events := append([]schema.DesktopSessionEvent(nil), tt.events...)
			d.resolveEventTimestamps(ctx, events)

			require.Len(t, events, len(tt.wantStart))
			for i := range events {
				require.Equal(t, tt.wantStart[i], events[i].StartTime, "event %d start", i)
				require.Equal(t, tt.wantEnd[i], events[i].EndTime, "event %d end", i)
				require.Equal(t, tt.wantIndices[i][0], events[i].StartScreenshotIndex, "event %d start idx", i)
				require.Equal(t, tt.wantIndices[i][1], events[i].EndScreenshotIndex, "event %d end idx", i)
			}
		})
	}
}

func TestDesktopSessionSummarizer_FlushChunkProviderError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("inference failed")
	provider := &countingProvider{imageErr: wantErr}
	d := newTestSummarizer(t, provider)
	d.chunk = append(d.chunk, []byte("png-bytes"))
	d.chunkStart = 5 * time.Second
	d.chunkEnd = 12 * time.Second

	require.NoError(t, d.flushChunk(t.Context()), "provider error must not abort the run")

	require.EqualValues(t, 1, provider.imageCalls.Load())
	require.True(t, d.screenshotAnalysisFailed)
	require.Empty(t, d.chunk, "chunk state must reset so the next chunk can be processed")
	require.Equal(t, time.Duration(0), d.chunkStart)
	require.Equal(t, time.Duration(0), d.chunkEnd)

	require.Len(t, d.allEvents, 1)
	failed := d.allEvents[0]
	require.Equal(t, desktop.FormatTimestamp(5*time.Second), failed.StartTime)
	require.Equal(t, desktop.FormatTimestamp(12*time.Second), failed.EndTime)
	require.Equal(t, wantErr.Error(), failed.InferenceErrorMessage)
	require.Equal(t, "high", failed.RiskLevel)
	require.Contains(t, failed.DetailedDescription, wantErr.Error())
}

func TestDesktopSessionSummarizer_FlushChunkContextCanceledPropagates(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	provider := &countingProvider{imageErr: context.Canceled}
	d := newTestSummarizer(t, provider)
	d.chunk = append(d.chunk, []byte("png-bytes"))

	require.ErrorIs(t, d.flushChunk(ctx), context.Canceled)
	require.False(t, d.screenshotAnalysisFailed, "ctx cancel must not synthesize a placeholder event")
	require.Empty(t, d.allEvents)
}

func TestDesktopSessionSummarizer_SynthesizeBumpsRiskOnPartialFailure(t *testing.T) {
	t.Parallel()

	d := newTestSummarizer(t, &countingProvider{})
	d.screenshotAnalysisFailed = true
	d.tooLarge = true

	analysis, err := d.synthesize(t.Context())
	require.NoError(t, err)
	require.True(t, analysis.TooLarge)
	require.True(t, analysis.ScreenshotAnalysisFailed)
	require.Equal(t, "medium", analysis.RiskLevel)
}

func TestDesktopSessionSummarizer_SynthesizeKeepsHigherRisk(t *testing.T) {
	t.Parallel()

	provider := &countingProvider{
		sessionAnalysis: &schema.DesktopSessionAnalysis{RiskLevel: "high"},
	}
	d := newTestSummarizer(t, provider)
	d.screenshotAnalysisFailed = true

	analysis, err := d.synthesize(t.Context())
	require.NoError(t, err)
	require.Equal(t, "high", analysis.RiskLevel, "must not downgrade existing higher risk")
}

func TestDesktopSessionSummarizer_AccumulateOversizeScreenshotEmitsPlaceholder(t *testing.T) {
	t.Parallel()

	d := newTestSummarizer(t, &countingProvider{})
	oversize := desktop.ScreenshotResult{
		PNG:       make([]byte, desktopMaxImageBytesPerChunk*2),
		Width:     10,
		Height:    10,
		StartTime: 3 * time.Second,
		EndTime:   4 * time.Second,
	}

	require.NoError(t, d.accumulate(t.Context(), oversize))
	require.Empty(t, d.chunk, "oversize screenshot must not be appended to the chunk")
	require.True(t, d.screenshotAnalysisFailed)
	require.Len(t, d.allEvents, 1)

	placeholder := d.allEvents[0]
	require.Equal(t, desktop.FormatTimestamp(oversize.StartTime), placeholder.StartTime)
	require.Equal(t, desktop.FormatTimestamp(oversize.EndTime), placeholder.EndTime)
	require.NotEmpty(t, placeholder.InferenceErrorMessage)
}

func TestDesktopSessionSummarizer_AccumulateFlushesPendingBeforeOversizeCheck(t *testing.T) {
	t.Parallel()

	smallPNG := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	provider := &countingProvider{}
	d := newTestSummarizer(t, provider)

	require.NoError(t, d.accumulate(t.Context(), desktop.ScreenshotResult{PNG: smallPNG, Width: 100, Height: 100}))
	oversize := desktop.ScreenshotResult{PNG: make([]byte, desktopMaxImageBytesPerChunk*2), Width: 10, Height: 10}
	require.NoError(t, d.accumulate(t.Context(), oversize))

	require.EqualValues(t, 1, provider.imageCalls.Load(), "pending chunk must flush before the oversize screenshot is dropped")
	require.True(t, d.screenshotAnalysisFailed)
}

func TestDesktopSessionSummarizer_SynthesizeTrimsToFitBudget(t *testing.T) {
	t.Parallel()

	d := newTestSummarizer(t, &countingProvider{})
	bigTitle := strings.Repeat("x", 1024)
	const events = 400
	d.allEvents = make([]schema.DesktopSessionEvent, events)
	for i := range d.allEvents {
		d.allEvents[i] = schema.DesktopSessionEvent{
			StartTime:        time.Duration(i).String(),
			EndTime:          time.Duration(i + 1).String(),
			TimelineTitle:    bigTitle,
			ShortDescription: bigTitle,
			RiskLevel:        "low",
		}
	}

	_, err := d.synthesize(t.Context())
	require.NoError(t, err)
	require.True(t, d.tooLarge, "synthesis must mark tooLarge when events were trimmed")
}

func TestConsolidateAdjacentEvents(t *testing.T) {
	t.Parallel()

	mk := func(start, end string, startIdx, endIdx int, category, title string) schema.DesktopSessionEvent {
		return schema.DesktopSessionEvent{
			Category:             category,
			StartTime:            start,
			EndTime:              end,
			StartScreenshotIndex: startIdx,
			EndScreenshotIndex:   endIdx,
			TimelineTitle:        title,
			ThreatCategory:       "none",
			RiskLevel:            "low",
			RiskScore:            10,
		}
	}

	tests := []struct {
		name      string
		in        []schema.DesktopSessionEvent
		wantTitle []string
		wantStart []string
		wantEnd   []string
	}{
		{
			name: "empty list unchanged",
			in:   nil,
		},
		{
			name:      "single event unchanged",
			in:        []schema.DesktopSessionEvent{mk("0:05", "0:10", 1, 3, "network", "Browsed YouTube")},
			wantTitle: []string{"Browsed YouTube"},
			wantStart: []string{"0:05"},
			wantEnd:   []string{"0:10"},
		},
		{
			name: "near-duplicate fireplace events collapse to one",
			in: []schema.DesktopSessionEvent{
				mk("1:39", "1:48", 5, 7, "network", "Watching fireplace video on YouTube"),
				mk("1:48", "1:56", 8, 10, "network", "Watching fireplace video with pre-roll ad"),
				mk("1:56", "2:07", 11, 13, "network", "Watching YouTube fireplace video with ads"),
				mk("2:07", "2:25", 14, 17, "network", "Watched fireplace YouTube video with ads"),
				mk("2:25", "2:29", 18, 19, "network", "Watching fireplace YouTube video"),
			},
			// Longest title wins after the merges; all five collapse since each adjacent pair Jaccards >= 0.5.
			wantTitle: []string{"Watching fireplace video with pre-roll ad"},
			wantStart: []string{"1:39"},
			wantEnd:   []string{"2:29"},
		},
		{
			name: "different categories never merge",
			in: []schema.DesktopSessionEvent{
				mk("0:05", "0:10", 1, 2, "network", "Browsed YouTube"),
				mk("0:10", "0:14", 3, 4, "process", "Browsed YouTube"),
			},
			wantTitle: []string{"Browsed YouTube", "Browsed YouTube"},
			wantStart: []string{"0:05", "0:10"},
			wantEnd:   []string{"0:10", "0:14"},
		},
		{
			name: "low title overlap leaves events distinct",
			in: []schema.DesktopSessionEvent{
				mk("0:05", "0:10", 1, 2, "network", "Browsed YouTube"),
				mk("0:10", "0:14", 3, 4, "network", "Visited GitHub repository"),
			},
			wantTitle: []string{"Browsed YouTube", "Visited GitHub repository"},
			wantStart: []string{"0:05", "0:10"},
			wantEnd:   []string{"0:10", "0:14"},
		},
		{
			name: "inference-error placeholder never merges",
			in: []schema.DesktopSessionEvent{
				mk("0:05", "0:10", 1, 2, "network", "Watching fireplace video on YouTube"),
				func() schema.DesktopSessionEvent {
					e := mk("0:10", "0:14", 3, 4, "network", "Watching fireplace YouTube video")
					e.InferenceErrorMessage = "chunk inference failed"
					return e
				}(),
				mk("0:14", "0:20", 5, 6, "network", "Watching fireplace video on YouTube"),
			},
			wantTitle: []string{
				"Watching fireplace video on YouTube",
				"Watching fireplace YouTube video",
				"Watching fireplace video on YouTube",
			},
			wantStart: []string{"0:05", "0:10", "0:14"},
			wantEnd:   []string{"0:10", "0:14", "0:20"},
		},
		{
			name: "merge spans through a chain of adjacent similar events",
			in: []schema.DesktopSessionEvent{
				mk("0:00", "0:05", 0, 1, "network", "Browsed YouTube"),
				mk("0:05", "0:10", 2, 3, "network", "Browsed YouTube videos"),
				mk("0:10", "0:14", 4, 4, "network", "Browsed YouTube"),
				mk("0:14", "0:20", 5, 7, "system_config", "Opened settings panel"),
			},
			wantTitle: []string{"Browsed YouTube videos", "Opened settings panel"},
			wantStart: []string{"0:00", "0:14"},
			wantEnd:   []string{"0:14", "0:20"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := consolidateAdjacentEvents(append([]schema.DesktopSessionEvent(nil), tt.in...))
			require.Len(t, got, len(tt.wantTitle))
			for i := range got {
				require.Equal(t, tt.wantTitle[i], got[i].TimelineTitle, "event %d title", i)
				require.Equal(t, tt.wantStart[i], got[i].StartTime, "event %d start", i)
				require.Equal(t, tt.wantEnd[i], got[i].EndTime, "event %d end", i)
			}
		})
	}
}

func TestConsolidateAdjacentEvents_UnionsIndicatorsAndTakesHigherRisk(t *testing.T) {
	t.Parallel()

	in := []schema.DesktopSessionEvent{
		{
			Category:        "network",
			TimelineTitle:   "Watching fireplace video on YouTube",
			StartTime:       "1:39",
			EndTime:         "1:48",
			RiskLevel:       "low",
			RiskScore:       15,
			ThreatCategory:  "none",
			SuspiciousFlags: []string{"watching ad-supported content"},
			VisibleURLs:     []string{"youtube.com"},
			Applications:    []string{"Chrome"},
		},
		{
			Category:            "network",
			TimelineTitle:       "Watching fireplace video on YouTube",
			StartTime:           "1:48",
			EndTime:             "2:29",
			RiskLevel:           "high",
			RiskScore:           70,
			ThreatCategory:      "exfiltration",
			SuspiciousFlags:     []string{"watching ad-supported content", "extremely long idle"},
			VisibleURLs:         []string{"youtube.com", "doubleclick.net"},
			Applications:        []string{"Chrome"},
			HasSensitiveData:    true,
			PrivilegeEscalation: true,
		},
	}

	got := consolidateAdjacentEvents(in)
	require.Len(t, got, 1)
	merged := got[0]

	require.Equal(t, "1:39", merged.StartTime, "earlier start kept")
	require.Equal(t, "2:29", merged.EndTime, "later end taken")
	require.Equal(t, "high", merged.RiskLevel, "higher risk level wins")
	require.Equal(t, 70, merged.RiskScore, "higher risk score wins")
	require.Equal(t, "exfiltration", merged.ThreatCategory, "specific threat category replaces 'none'")
	require.ElementsMatch(t,
		[]string{"watching ad-supported content", "extremely long idle"},
		merged.SuspiciousFlags,
	)
	require.ElementsMatch(t, []string{"youtube.com", "doubleclick.net"}, merged.VisibleURLs)
	require.ElementsMatch(t, []string{"Chrome"}, merged.Applications)
	require.True(t, merged.HasSensitiveData)
	require.True(t, merged.PrivilegeEscalation)
	require.False(t, merged.DataExfiltration)
}

func TestTitleSimilarity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		a, b    string
		wantMin float64
		wantMax float64
	}{
		{name: "identical", a: "Browsed YouTube", b: "Browsed YouTube", wantMin: 0.99, wantMax: 1.0},
		{name: "disjoint", a: "Browsed YouTube", b: "Edited /etc/hosts", wantMin: 0, wantMax: 0},
		{name: "stopwords only differ", a: "Watching video on YouTube", b: "Watching video YouTube", wantMin: 0.99, wantMax: 1.0},
		{name: "moderate overlap", a: "Watching fireplace video on YouTube", b: "Watched fireplace YouTube video with ads", wantMin: 0.4, wantMax: 0.7},
		{name: "empty a", a: "", b: "x", wantMin: 0, wantMax: 0},
		{name: "empty b", a: "x", b: "", wantMin: 0, wantMax: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := titleSimilarity(tt.a, tt.b)
			require.GreaterOrEqual(t, got, tt.wantMin, "got %f", got)
			require.LessOrEqual(t, got, tt.wantMax, "got %f", got)
		})
	}
}

func TestStreamScreenshots_FlushForwardsAllResults(t *testing.T) {
	t.Parallel()

	processor := &fakeProcessor{flushResults: []desktop.ScreenshotResult{
		{PNG: []byte{0x01}, Width: 10, Height: 10},
		{PNG: []byte{0x02, 0x03}, Width: 20, Height: 20},
	}}

	eventsCh := make(chan apievents.AuditEvent)
	errCh := make(chan error)
	out := make(chan desktop.ScreenshotResult, 2)

	close(eventsCh)
	close(errCh)

	stats := streamScreenshots(t.Context(), processor, eventsCh, errCh, out)
	require.NoError(t, stats.err)
	require.Equal(t, 2, stats.totalScreenshots)
	require.Equal(t, 3, stats.totalBytes)

	first, ok := <-out
	require.True(t, ok)
	require.Equal(t, []byte{0x01}, first.PNG)

	second, ok := <-out
	require.True(t, ok)
	require.Equal(t, []byte{0x02, 0x03}, second.PNG)
}

func TestStreamScreenshots_FlushOnlyWhenBothChannelsClose(t *testing.T) {
	t.Parallel()

	flushPNG := []byte{0x01}
	processor := &fakeProcessor{flushResults: []desktop.ScreenshotResult{{PNG: flushPNG, Width: 50, Height: 50}}}

	eventsCh := make(chan apievents.AuditEvent)
	errCh := make(chan error)
	out := make(chan desktop.ScreenshotResult, 1)

	close(eventsCh)
	close(errCh)

	stats := streamScreenshots(t.Context(), processor, eventsCh, errCh, out)

	require.NoError(t, stats.err)
	require.Equal(t, 1, stats.totalScreenshots)
	require.Equal(t, len(flushPNG), stats.totalBytes)

	got, ok := <-out
	require.True(t, ok)
	require.Equal(t, flushPNG, got.PNG)
}

func TestStreamScreenshots_ErrChClosingDoesNotStrandLoop(t *testing.T) {
	t.Parallel()

	processor := &fakeProcessor{}

	eventsCh := make(chan apievents.AuditEvent)
	errCh := make(chan error)
	out := make(chan desktop.ScreenshotResult, 1)

	// errCh closes first with no error; the loop must keep draining eventsCh and exit when it closes too.
	close(errCh)

	done := make(chan screenshotStreamStats, 1)
	go func() {
		done <- streamScreenshots(t.Context(), processor, eventsCh, errCh, out)
	}()

	select {
	case <-done:
		t.Fatal("streamScreenshots returned before eventsCh closed")
	case <-time.After(50 * time.Millisecond):
	}

	close(eventsCh)

	select {
	case stats := <-done:
		require.NoError(t, stats.err)
	case <-time.After(time.Second):
		t.Fatal("streamScreenshots blocked after eventsCh closed")
	}
}

func TestStreamScreenshots_ContextCancelExits(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	processor := &fakeProcessor{}

	eventsCh := make(chan apievents.AuditEvent)
	errCh := make(chan error)
	out := make(chan desktop.ScreenshotResult, 1)

	done := make(chan screenshotStreamStats, 1)
	go func() {
		done <- streamScreenshots(ctx, processor, eventsCh, errCh, out)
	}()

	cancel()

	select {
	case stats := <-done:
		require.ErrorIs(t, stats.err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("streamScreenshots did not exit on context cancel")
	}
}

func TestStreamScreenshots_ProcessEventError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("decode failed")
	processor := &fakeProcessor{processErr: wantErr}

	eventsCh := make(chan apievents.AuditEvent, 1)
	errCh := make(chan error)
	out := make(chan desktop.ScreenshotResult, 1)

	eventsCh <- newFakeAuditEvent()
	close(eventsCh)
	close(errCh)

	stats := streamScreenshots(t.Context(), processor, eventsCh, errCh, out)
	require.ErrorIs(t, stats.err, wantErr)
	require.Equal(t, 0, stats.totalScreenshots)
}

func TestStreamScreenshots_ErrChDeliversError(t *testing.T) {
	t.Parallel()

	processor := &fakeProcessor{}
	wantErr := errors.New("stream broke")

	eventsCh := make(chan apievents.AuditEvent)
	errCh := make(chan error, 1)
	out := make(chan desktop.ScreenshotResult, 1)

	errCh <- wantErr
	close(errCh)
	close(eventsCh)

	stats := streamScreenshots(t.Context(), processor, eventsCh, errCh, out)
	require.ErrorIs(t, stats.err, wantErr)
}

func TestStreamScreenshots_FlushError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("flush failed")
	processor := &fakeProcessor{flushErr: wantErr}

	eventsCh := make(chan apievents.AuditEvent)
	errCh := make(chan error)
	out := make(chan desktop.ScreenshotResult, 1)

	close(eventsCh)
	close(errCh)

	stats := streamScreenshots(t.Context(), processor, eventsCh, errCh, out)
	require.ErrorIs(t, stats.err, wantErr)
}

func TestCountClaudeImageTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		width  int
		height int
		want   int
	}{
		{
			name:   "zero",
			width:  0,
			height: 0,
			want:   0,
		},
		{
			name:   "below limit, square",
			width:  750,
			height: 750,
			want:   750 * 750 / claudeImagePixelsPerToken,
		},
		{
			name:   "below limit, wide",
			width:  1000,
			height: 500,
			want:   1000 * 500 / claudeImagePixelsPerToken,
		},
		{
			name:   "exactly at limit",
			width:  claudeMaxImageDimension,
			height: claudeMaxImageDimension,
			want:   claudeMaxImageDimension * claudeMaxImageDimension / claudeImagePixelsPerToken,
		},
		{
			name:   "scales when width exceeds limit",
			width:  3136, // 2x of claudeMaxImageDimension
			height: 1568,
			want:   1568 * 784 / claudeImagePixelsPerToken,
		},
		{
			name:   "scales when height exceeds limit",
			width:  1568,
			height: 3136,
			want:   784 * 1568 / claudeImagePixelsPerToken,
		},
		{
			name:   "scales when both exceed limit, longest edge wins",
			width:  3136,
			height: 4704, // 3x of claudeMaxImageDimension
			want:   (1045 * 1568) / claudeImagePixelsPerToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, countClaudeImageTokens(tt.width, tt.height))
		})
	}
}

func newTestSummarizer(t *testing.T, provider InferenceProvider) *desktopSessionSummarizer {
	t.Helper()

	return &desktopSessionSummarizer{
		logger:  slog.New(slog.DiscardHandler),
		details: sessionDetails{sessionID: "test", provider: provider},
		chunk:   make([][]byte, 0, desktopMaxImagesPerChunk),
	}
}

type countingProvider struct {
	mockSessionInferenceProvider
	imageCalls      atomic.Int32
	analysis        *schema.DesktopScreenshotAnalysis
	sessionAnalysis *schema.DesktopSessionAnalysis
	imageErr        error
}

func (p *countingProvider) Summarize(
	context.Context, session.ID, string, io.ReadCloser,
) (string, error) {
	return "", nil
}

func (p *countingProvider) SummarizeMultipleImages(
	context.Context, session.ID, string, []schema.ImageData,
) (*schema.DesktopScreenshotAnalysis, error) {
	p.imageCalls.Add(1)
	if p.imageErr != nil {
		return nil, p.imageErr
	}
	if p.analysis != nil {
		return p.analysis, nil
	}

	return &schema.DesktopScreenshotAnalysis{}, nil
}

func (p *countingProvider) SummarizeDesktopSession(
	context.Context, session.ID, string, string,
) (*schema.DesktopSessionAnalysis, error) {
	if p.sessionAnalysis != nil {
		// Return a copy so tests asserting struct mutation observe only the synthesizer's writes.
		out := *p.sessionAnalysis
		return &out, nil
	}

	return &schema.DesktopSessionAnalysis{}, nil
}

func (p *countingProvider) GetTotalTokens() (uint64, uint64) { return 0, 0 }
func (p *countingProvider) GetType() string                  { return "test" }

func (p *countingProvider) CondenseForEmbedding(
	context.Context, *summarizerv1pb.Summary,
) (string, error) {
	return "", nil
}

type fakeProcessor struct {
	processResults []*desktop.ScreenshotResult
	processErr     error

	flushResults []desktop.ScreenshotResult
	flushErr     error

	processCalls int
}

func (f *fakeProcessor) ProcessEvent(apievents.AuditEvent) (*desktop.ScreenshotResult, error) {
	if f.processErr != nil {
		return nil, f.processErr
	}
	if f.processCalls < len(f.processResults) {
		r := f.processResults[f.processCalls]
		f.processCalls++

		return r, nil
	}

	return nil, nil
}

func (f *fakeProcessor) Flush() ([]desktop.ScreenshotResult, error) {
	if f.flushErr != nil {
		return nil, f.flushErr
	}

	return f.flushResults, nil
}

func newFakeAuditEvent() apievents.AuditEvent {
	return &apievents.DesktopRecording{
		Metadata: apievents.Metadata{Time: time.Unix(0, 0)},
	}
}
