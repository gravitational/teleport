package desktop

import (
	"bytes"
	"image"
	"image/png"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv/desktop/rdpstate/rdpstatetest"
)

func TestSampleImageHash(t *testing.T) {
	t.Parallel()

	t.Run("nil returns 0", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, uint64(0), sampleImageHash(nil))
	})

	t.Run("zero-size returns 0", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, uint64(0), sampleImageHash(image.NewRGBA(image.Rect(0, 0, 0, 0))))
	})

	t.Run("deterministic on identical pixels", func(t *testing.T) {
		t.Parallel()

		a := newFilledRGBA(t, image.Rect(0, 0, 200, 200), 0x42)
		b := newFilledRGBA(t, image.Rect(0, 0, 200, 200), 0x42)
		require.Equal(t, sampleImageHash(a), sampleImageHash(b))
	})

	t.Run("differs when pixels differ", func(t *testing.T) {
		t.Parallel()

		a := newFilledRGBA(t, image.Rect(0, 0, 200, 200), 0x42)
		b := newFilledRGBA(t, image.Rect(0, 0, 200, 200), 0x42)
		b.Pix[0] ^= 0xFF

		require.NotEqual(t, sampleImageHash(a), sampleImageHash(b))
	})

	t.Run("smaller-than-sample dimensions do not panic", func(t *testing.T) {
		t.Parallel()

		dims := []image.Rectangle{
			image.Rect(0, 0, 1, 1),
			image.Rect(0, 0, hashSampleCount-1, hashSampleCount*4),
			image.Rect(0, 0, hashSampleCount*4, hashSampleCount-1),
			image.Rect(0, 0, hashSampleCount-1, hashSampleCount-1),
		}
		for _, r := range dims {
			require.NotPanics(t, func() {
				sampleImageHash(newFilledRGBA(t, r, 0x80))
			}, "bounds=%v", r)
		}
	})

	t.Run("non-zero origin is handled", func(t *testing.T) {
		t.Parallel()

		a := newFilledRGBA(t, image.Rect(0, 0, 100, 100), 0x33)
		b := newFilledRGBA(t, image.Rect(50, 60, 150, 160), 0x33)
		require.Equal(t, sampleImageHash(a), sampleImageHash(b))
	})
}

func newFilledRGBA(t *testing.T, r image.Rectangle, v uint8) *image.RGBA {
	t.Helper()
	img := image.NewRGBA(r)
	for i := range img.Pix {
		img.Pix[i] = v
	}
	return img
}

func TestIsActiveWindowSignificant(t *testing.T) {
	t.Parallel()

	const screenArea = 1000 * 1000

	tests := []struct {
		name string
		r    image.Rectangle
		want bool
	}{
		{name: "empty is not significant", r: image.Rectangle{}, want: false},
		{name: "below threshold is noise", r: image.Rect(0, 0, 100, 100), want: false},
		{name: "exactly at threshold is significant", r: image.Rect(0, 0, 1000, 100), want: true},
		{name: "above threshold is significant", r: image.Rect(0, 0, 500, 500), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, isActiveWindowSignificant(tt.r, screenArea))
		})
	}
}

func TestChooseCropBounds(t *testing.T) {
	t.Parallel()

	bounds := image.Rect(0, 0, 1000, 1000)

	tests := []struct {
		name            string
		activeWindow    image.Rectangle
		forceFullScreen bool
		want            image.Rectangle
	}{
		{
			name:            "force full screen returns bounds even with valid active window",
			activeWindow:    image.Rect(100, 100, 600, 600),
			forceFullScreen: true,
			want:            bounds,
		},
		{
			name:         "empty active window returns bounds",
			activeWindow: image.Rectangle{},
			want:         bounds,
		},
		{
			name:         "active window below minimum fraction returns bounds",
			activeWindow: image.Rect(0, 0, 100, 100),
			want:         bounds,
		},
		{
			name:         "active window padded exceeds screen area returns bounds",
			activeWindow: image.Rect(50, 50, 950, 950),
			want:         bounds,
		},
		{
			name:         "valid active window returns padded rectangle",
			activeWindow: image.Rect(100, 100, 500, 500),
			want:         image.Rect(0, 0, 600, 600),
		},
		{
			name:         "padding clamps to bounds on the top/left edge",
			activeWindow: image.Rect(0, 0, 400, 400),
			want:         image.Rect(0, 0, 500, 500),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := chooseCropBounds(bounds, tt.activeWindow, tt.forceFullScreen)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestRectFraction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		r         image.Rectangle
		totalArea int
		want      float64
	}{
		{name: "empty rectangle returns 0", r: image.Rectangle{}, totalArea: 100, want: 0},
		{name: "zero total area returns 0", r: image.Rect(0, 0, 10, 10), totalArea: 0, want: 0},
		{name: "negative total area returns 0", r: image.Rect(0, 0, 10, 10), totalArea: -1, want: 0},
		{name: "half of total", r: image.Rect(0, 0, 10, 10), totalArea: 200, want: 0.5},
		{name: "full", r: image.Rect(0, 0, 10, 10), totalArea: 100, want: 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := rectFraction(tt.r, tt.totalArea)
			require.InDelta(t, tt.want, got, 1e-9)
		})
	}
}

func TestPadRect(t *testing.T) {
	t.Parallel()

	maxBounds := image.Rect(0, 0, 100, 100)

	tests := []struct {
		name    string
		r       image.Rectangle
		padding int
		want    image.Rectangle
	}{
		{
			name:    "padding fits within bounds",
			r:       image.Rect(40, 40, 60, 60),
			padding: 10,
			want:    image.Rect(30, 30, 70, 70),
		},
		{
			name:    "padding clamps to bounds on all sides",
			r:       image.Rect(10, 10, 90, 90),
			padding: 50,
			want:    image.Rect(0, 0, 100, 100),
		},
		{
			name:    "zero padding returns intersected rectangle",
			r:       image.Rect(10, 10, 90, 90),
			padding: 0,
			want:    image.Rect(10, 10, 90, 90),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := padRect(tt.r, tt.padding, maxBounds)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestFormatTimestamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{name: "zero", d: 0, want: "0:00"},
		{name: "5 seconds", d: 5 * time.Second, want: "0:05"},
		{name: "59 seconds", d: 59 * time.Second, want: "0:59"},
		{name: "65 seconds", d: 65 * time.Second, want: "1:05"},
		{name: "exactly one hour", d: time.Hour, want: "1:00:00"},
		{name: "hours minutes seconds", d: time.Hour + 2*time.Minute + 3*time.Second, want: "1:02:03"},
		{name: "sub-second truncates", d: 999 * time.Millisecond, want: "0:00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, FormatTimestamp(tt.d))
		})
	}
}

func TestFormatDurationLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		start time.Duration
		end   time.Duration
		want  string
	}{
		{
			name:  "single timestamp when start equals end",
			start: 5 * time.Second,
			end:   5 * time.Second,
			want:  "Screenshot taken at 0:05",
		},
		{
			name:  "range when start differs from end",
			start: 5 * time.Second,
			end:   15 * time.Second,
			want:  "Screenshot taken at 0:05 - 0:15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, formatDurationLabel(tt.start, tt.end))
		})
	}
}

func TestExpandCropForTimestamp(t *testing.T) {
	t.Parallel()

	screenBounds := image.Rect(0, 0, 1000, 1000)

	tests := []struct {
		name     string
		crop     image.Rectangle
		minWidth int
		want     image.Rectangle
	}{
		{
			name:     "already wide enough is unchanged",
			crop:     image.Rect(400, 100, 700, 200),
			minWidth: 200,
			want:     image.Rect(400, 100, 700, 200),
		},
		{
			name:     "expands centered when there is room",
			crop:     image.Rect(400, 100, 500, 200),
			minWidth: 200,
			want:     image.Rect(350, 100, 550, 200),
		},
		{
			name:     "shifts right when expansion overflows left edge",
			crop:     image.Rect(0, 100, 100, 200),
			minWidth: 200,
			want:     image.Rect(0, 100, 200, 200),
		},
		{
			name:     "shifts left when expansion overflows right edge",
			crop:     image.Rect(900, 100, 1000, 200),
			minWidth: 200,
			want:     image.Rect(800, 100, 1000, 200),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := expandCropForTimestamp(tt.crop, tt.minWidth, screenBounds)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestFitImageDimensions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		w, h  int
		wantW int
		wantH int
	}{
		{
			name:  "within both budgets is unchanged",
			w:     800,
			h:     600,
			wantW: 800,
			wantH: 600,
		},
		{
			name:  "width exceeds max dimension scales down",
			w:     3000,
			h:     1000,
			wantW: screenshotMaxDimension,
			wantH: (1000+timestampBarHeight)*screenshotMaxDimension/3000 - timestampBarHeight,
		},
		{
			name:  "total height exceeds max dimension scales down",
			w:     800,
			h:     2000,
			wantW: 800 * screenshotMaxDimension / (2000 + timestampBarHeight),
			wantH: screenshotMaxDimension - timestampBarHeight,
		},
		{
			// 10000x1 scales to 1568x4 by long-edge fitting, leaving 4-bar=-26 which the function clamps to 1.
			name:  "very wide and short image clamps h to 1",
			w:     10000,
			h:     1,
			wantW: screenshotMaxDimension,
			wantH: 1,
		},
		{
			// 1x10000: long-edge fit would truncate w to 0; the clamp keeps it at 1.
			name:  "very tall and narrow image clamps w to 1",
			w:     1,
			h:     10000,
			wantW: 1,
			wantH: screenshotMaxDimension - timestampBarHeight,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotW, gotH := fitImageDimensions(tt.w, tt.h)
			require.Equal(t, tt.wantW, gotW)
			require.Equal(t, tt.wantH, gotH)
		})
	}
}

func TestFitImageDimensions_RespectsPixelBudget(t *testing.T) {
	t.Parallel()

	// Under the dimension limit but over the pixel budget.
	gotW, gotH := fitImageDimensions(1500, 1500)
	pixels := gotW * (gotH + timestampBarHeight)
	require.LessOrEqual(t, pixels, screenshotMaxPixels,
		"final pixel count must stay within screenshotMaxPixels")
}

func TestDesktopRecordingProcessor_AdaptiveInterval(t *testing.T) {
	t.Parallel()

	start := time.Unix(1_700_000_000, 0)

	tests := []struct {
		name             string
		hadActiveWindow  bool
		screenshotsTaken int
		elapsed          time.Duration
		want             time.Duration
	}{
		{
			name:             "active window, early session",
			hadActiveWindow:  true,
			screenshotsTaken: 5,
			want:             baseScreenshotInterval / 2,
		},
		{
			name:             "active window, under 2 minutes",
			hadActiveWindow:  true,
			screenshotsTaken: 100,
			elapsed:          time.Minute,
			want:             baseScreenshotInterval / 2,
		},
		{
			name:             "active window, under 5 minutes",
			hadActiveWindow:  true,
			screenshotsTaken: 100,
			elapsed:          3 * time.Minute,
			want:             baseScreenshotInterval,
		},
		{
			name:             "active window, under 10 minutes",
			hadActiveWindow:  true,
			screenshotsTaken: 100,
			elapsed:          7 * time.Minute,
			want:             5 * time.Second,
		},
		{
			name:             "active window, long session",
			hadActiveWindow:  true,
			screenshotsTaken: 100,
			elapsed:          15 * time.Minute,
			want:             7 * time.Second,
		},
		{
			name:             "no active window, early session",
			screenshotsTaken: 5,
			want:             baseScreenshotInterval,
		},
		{
			name:             "no active window, under 2 minutes",
			screenshotsTaken: 100,
			elapsed:          time.Minute,
			want:             baseScreenshotInterval,
		},
		{
			name:             "no active window, under 5 minutes",
			screenshotsTaken: 100,
			elapsed:          3 * time.Minute,
			want:             7 * time.Second,
		},
		{
			name:             "no active window, under 10 minutes",
			screenshotsTaken: 100,
			elapsed:          7 * time.Minute,
			want:             10 * time.Second,
		},
		{
			name:             "no active window, long session",
			screenshotsTaken: 100,
			elapsed:          15 * time.Minute,
			want:             15 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := &RecordingProcessor{
				startTime:        start,
				hadActiveWindow:  tt.hadActiveWindow,
				screenshotsTaken: tt.screenshotsTaken,
			}
			require.Equal(t, tt.want, d.adaptiveInterval(start.Add(tt.elapsed)))
		})
	}
}

func TestDesktopRecordingProcessor_ShouldTriggerSettled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		quietGap   time.Duration
		pduBytes   int
		wantResult bool
	}{
		{name: "no quiet period and no activity", quietGap: 0, pduBytes: 0, wantResult: false},
		{name: "quiet period but no activity", quietGap: time.Second, pduBytes: 0, wantResult: false},
		{name: "activity but too short a gap", quietGap: 100 * time.Millisecond, pduBytes: 100, wantResult: false},
		{name: "exactly at threshold with activity", quietGap: settledQuietPeriod, pduBytes: 1, wantResult: true},
		{name: "past threshold with activity", quietGap: time.Second, pduBytes: 100, wantResult: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := &RecordingProcessor{
				quietGap:          tt.quietGap,
				pduBytesSinceLast: tt.pduBytes,
			}
			require.Equal(t, tt.wantResult, d.shouldTriggerSettled())
		})
	}
}

func TestDesktopRecordingProcessor_ShouldTriggerTimeInterval(t *testing.T) {
	t.Parallel()

	start := time.Unix(1_700_000_000, 0)

	tests := []struct {
		name            string
		lastScreenshot  time.Time
		eventTime       time.Time
		wantResult      bool
		hadActiveWindow bool
	}{
		{
			name:       "no previous screenshot fires immediately",
			eventTime:  start,
			wantResult: true,
		},
		{
			name:           "within adaptive interval does not fire",
			lastScreenshot: start,
			eventTime:      start.Add(time.Second),
			wantResult:     false,
		},
		{
			name:           "past adaptive interval fires",
			lastScreenshot: start,
			eventTime:      start.Add(5 * time.Second),
			wantResult:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := &RecordingProcessor{
				startTime:          start,
				lastScreenshotTime: tt.lastScreenshot,
				hadActiveWindow:    tt.hadActiveWindow,
			}
			require.Equal(t, tt.wantResult, d.shouldTriggerTimeInterval(tt.eventTime))
		})
	}
}

func TestDesktopRecordingProcessor_EvaluateTrigger(t *testing.T) {
	t.Parallel()

	start := time.Unix(1_700_000_000, 0)

	t.Run("first capture forces full-screen", func(t *testing.T) {
		t.Parallel()

		d := &RecordingProcessor{startTime: start}
		capture, full := d.evaluateTrigger(start)
		require.True(t, capture)
		require.True(t, full)
	})

	t.Run("settled trigger captures without forcing full-screen and resets quiet gap", func(t *testing.T) {
		t.Parallel()

		d := &RecordingProcessor{
			startTime:          start,
			lastScreenshotTime: start,
			quietGap:           settledQuietPeriod,
			pduBytesSinceLast:  100,
		}
		capture, full := d.evaluateTrigger(start.Add(time.Second))
		require.True(t, capture)
		require.False(t, full)
		require.Equal(t, time.Duration(0), d.quietGap, "quietGap should reset after settled fires")
	})

	t.Run("time interval trigger captures without forcing full-screen", func(t *testing.T) {
		t.Parallel()

		d := &RecordingProcessor{
			startTime:          start,
			lastScreenshotTime: start,
		}
		capture, full := d.evaluateTrigger(start.Add(10 * time.Second))
		require.True(t, capture)
		require.False(t, full)
	})

	t.Run("no trigger when recent capture and no activity", func(t *testing.T) {
		t.Parallel()

		d := &RecordingProcessor{
			startTime:          start,
			lastScreenshotTime: start,
		}
		capture, full := d.evaluateTrigger(start.Add(100 * time.Millisecond))
		require.False(t, capture)
		require.False(t, full)
	})
}

func TestDesktopRecordingProcessor_Release(t *testing.T) {
	t.Parallel()

	d := NewRecordingProcessor(NewGlyphCache())

	require.NotPanics(t, func() {
		d.Release()
		d.Release()
	}, "Release must be safe to call repeatedly")
}

func TestDesktopRecordingProcessor_ProcessEvent_PropagatesHandleMessageError(t *testing.T) {
	t.Parallel()

	d := NewRecordingProcessor(NewGlyphCache())
	defer d.Release()

	bad := rdpstatetest.TDPBEvent([]byte{0xFF, 0xFF, 0xFF})
	bad.Metadata = apievents.Metadata{Time: time.Unix(1_700_000_000, 0).UTC()}

	_, err := d.ProcessEvent(bad)
	require.Error(t, err)
	require.ErrorContains(t, err, "processing desktop recording event")
}

func TestDesktopRecordingProcessor_HandleSessionStart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		evt  func(time.Time) apievents.AuditEvent
	}{
		{
			name: "windows",
			evt: func(ts time.Time) apievents.AuditEvent {
				return &apievents.WindowsDesktopSessionStart{
					Metadata: apievents.Metadata{Time: ts},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := NewRecordingProcessor(NewGlyphCache())
			defer d.Release()

			want := time.Unix(1_700_000_000, 0).UTC()
			_, err := d.ProcessEvent(tt.evt(want))
			require.NoError(t, err)
			require.Equal(t, want, d.StartTime())
		})
	}
}

func TestDesktopRecordingProcessor_Flush_NoPending(t *testing.T) {
	t.Parallel()

	d := NewRecordingProcessor(NewGlyphCache())
	defer d.Release()

	results, err := d.Flush()
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestDesktopRecordingProcessor_Flush_EmitsPending(t *testing.T) {
	t.Parallel()

	d := NewRecordingProcessor(NewGlyphCache())
	defer d.Release()

	screen := image.Rect(0, 0, 400, 300)
	img := image.NewRGBA(screen)
	for i := range img.Pix {
		img.Pix[i] = 0x80
	}

	d.pending = &pendingScreenshot{
		img:        img,
		cropBounds: screen,
		hash:       42,
		startTime:  3 * time.Second,
		endTime:    9 * time.Second,
		fullScreen: true,
	}

	results, err := d.Flush()
	require.NoError(t, err)
	require.Len(t, results, 1)

	result := results[0]
	require.Equal(t, 3*time.Second, result.StartTime)
	require.Equal(t, 9*time.Second, result.EndTime)
	require.NotEmpty(t, result.PNG)

	decoded, err := png.Decode(bytes.NewReader(result.PNG))
	require.NoError(t, err)
	require.Equal(t, screen.Dx(), decoded.Bounds().Dx())
	require.Equal(t, screen.Dy()+timestampBarHeight, decoded.Bounds().Dy())

	require.Nil(t, d.pending, "Flush should clear pending state")

	second, err := d.Flush()
	require.NoError(t, err)
	require.Empty(t, second)
}

func FuzzProcessEvent(f *testing.F) {
	// screenW, screenH, bmpLeft, bmpTop, bmpW, bmpH, pixel, cursorX, cursorY
	f.Add(uint16(800), uint16(600), int16(0), int16(0), uint8(50), uint8(50), uint16(rdpstatetest.RGB565Red), int16(10), int16(10))
	f.Add(uint16(800), uint16(600), int16(700), int16(500), uint8(200), uint8(200), uint16(rdpstatetest.RGB565White), int16(0), int16(0))
	f.Add(uint16(100), uint16(100), int16(-10), int16(-10), uint8(50), uint8(50), uint16(rdpstatetest.RGB565Blue), int16(99), int16(99))

	ts := time.Unix(1_700_000_000, 0).UTC()
	f.Fuzz(func(t *testing.T,
		screenW, screenH uint16,
		bmpLeft, bmpTop int16, bmpW, bmpH uint8, pixel uint16,
		cursorX, cursorY int16,
	) {
		sw := uint32(screenW)%2048 + 1
		sh := uint32(screenH)%2048 + 1

		d := NewRecordingProcessor(NewGlyphCache())
		defer d.Release()

		hello, err := rdpstatetest.EncodeTDPBServerHello(sw, sh)
		if err != nil {
			return
		}
		hello.Metadata = apievents.Metadata{Time: ts}
		if _, err := d.ProcessEvent(hello); err != nil {
			return
		}

		bitmap, err := rdpstatetest.EncodeTDPBFastPathPDU(
			rdpstatetest.BuildBitmapPDU(int(bmpLeft), int(bmpTop), int(bmpW), int(bmpH), pixel),
		)
		if err != nil {
			return
		}
		bitmap.Metadata = apievents.Metadata{Time: ts.Add(time.Second)}
		_, _ = d.ProcessEvent(bitmap)

		cursor, err := rdpstatetest.EncodeTDPBFastPathPDU(
			rdpstatetest.BuildPointerPositionPDU(int(cursorX), int(cursorY)),
		)
		if err != nil {
			return
		}
		cursor.Metadata = apievents.Metadata{Time: ts.Add(2 * time.Second)}
		_, _ = d.ProcessEvent(cursor)
	})
}

func FuzzSampleImageHash(f *testing.F) {
	f.Add(uint8(0), uint8(0), []byte{})
	f.Add(uint8(1), uint8(1), []byte{0, 0, 0, 0})
	f.Add(uint8(hashSampleCount-1), uint8(hashSampleCount*2), []byte{})
	f.Add(uint8(hashSampleCount*2), uint8(hashSampleCount-1), []byte{})
	f.Add(uint8(hashSampleCount), uint8(hashSampleCount), []byte{})

	f.Fuzz(func(t *testing.T, w, h uint8, pix []byte) {
		r := image.Rect(0, 0, int(w), int(h))
		need := int(w) * int(h) * bytesPerPixel
		if need == 0 {
			sampleImageHash(image.NewRGBA(r))
			return
		}
		if len(pix) < need {
			pix = append(pix, make([]byte, need-len(pix))...)
		}
		img := &image.RGBA{
			Pix:    pix[:need],
			Stride: int(w) * bytesPerPixel,
			Rect:   r,
		}
		_ = sampleImageHash(img)
	})
}
