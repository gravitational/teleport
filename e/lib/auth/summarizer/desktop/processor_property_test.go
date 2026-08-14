package desktop

import (
	"image"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestProperty_ChooseCropBounds_WithinScreen(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		bounds := genScreenRect(t)
		activeWindow := genRect(t, "window")
		force := rapid.Bool().Draw(t, "force")

		got := chooseCropBounds(bounds, activeWindow, force)
		require.True(t, got.In(bounds) || got == bounds || got.Empty(),
			"got=%v not within bounds=%v active=%v force=%v", got, bounds, activeWindow, force)
	})
}

func TestProperty_ChooseCropBounds_ForceReturnsBounds(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		bounds := genScreenRect(t)
		activeWindow := genRect(t, "window")

		require.Equal(t, bounds, chooseCropBounds(bounds, activeWindow, true))
	})
}

func TestProperty_ChooseCropBounds_AreaNoMoreThanBounds(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		bounds := genScreenRect(t)
		activeWindow := genRect(t, "window")
		force := rapid.Bool().Draw(t, "force")

		got := chooseCropBounds(bounds, activeWindow, force)
		require.LessOrEqual(t, got.Dx()*got.Dy(), bounds.Dx()*bounds.Dy(),
			"got area %d > bounds area %d (got=%v bounds=%v)",
			got.Dx()*got.Dy(), bounds.Dx()*bounds.Dy(), got, bounds)
	})
}

func TestProperty_IsActiveWindowSignificant_EmptyIsFalse(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		screenArea := rapid.IntRange(0, 100_000_000).Draw(t, "area")
		require.False(t, isActiveWindowSignificant(image.Rectangle{}, screenArea))
	})
}

func TestProperty_IsActiveWindowSignificant_MonotonicInArea(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		screenArea := rapid.IntRange(minActiveWindowFraction, 100_000_000).Draw(t, "area")

		w1 := rapid.IntRange(1, 1000).Draw(t, "w1")
		h1 := rapid.IntRange(1, 1000).Draw(t, "h1")
		extraW := rapid.IntRange(0, 1000).Draw(t, "extraW")
		extraH := rapid.IntRange(0, 1000).Draw(t, "extraH")
		r1 := image.Rect(0, 0, w1, h1)
		r2 := image.Rect(0, 0, w1+extraW, h1+extraH)

		if isActiveWindowSignificant(r1, screenArea) {
			require.True(t, isActiveWindowSignificant(r2, screenArea),
				"monotonicity violated: r1=%v r2=%v area=%d", r1, r2, screenArea)
		}
	})
}

func TestProperty_PadRect_WithinMaxBounds(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		r := genRect(t, "r")
		padding := rapid.IntRange(0, 10_000).Draw(t, "padding")
		maxBounds := genRect(t, "max")

		got := padRect(r, padding, maxBounds)
		require.True(t, got.In(maxBounds) || got.Empty(),
			"got=%v not within max=%v r=%v padding=%d", got, maxBounds, r, padding)
	})
}

func TestProperty_PadRect_ContainsOriginalIntersection(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		r := genRect(t, "r")
		padding := rapid.IntRange(0, 10_000).Draw(t, "padding")
		maxBounds := genRect(t, "max")

		got := padRect(r, padding, maxBounds)
		intersect := r.Intersect(maxBounds)
		if intersect.Empty() {
			return
		}

		require.True(t, intersect.In(got),
			"got=%v doesn't contain intersect=%v (r=%v padding=%d max=%v)",
			got, intersect, r, padding, maxBounds)
	})
}

func TestProperty_PadRect_ZeroPaddingEqualsIntersection(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		r := genRect(t, "r")
		maxBounds := genRect(t, "max")
		require.Equal(t, r.Intersect(maxBounds), padRect(r, 0, maxBounds))
	})
}

func TestProperty_RectFraction_InZeroToOne(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		r := genRect(t, "r")

		totalArea := r.Dx() * r.Dy()
		if totalArea <= 0 {
			totalArea = rapid.IntRange(1, 100_000_000).Draw(t, "ta")
		} else {
			extra := rapid.IntRange(0, 100_000_000).Draw(t, "extra")
			totalArea += extra
		}

		got := rectFraction(r, totalArea)
		require.GreaterOrEqual(t, got, 0.0)
		require.LessOrEqual(t, got, 1.0)
	})
}

func TestProperty_RectFraction_EmptyOrNonPositiveAreaIsZero(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		emptyR := image.Rectangle{}
		require.InDelta(t, 0.0, rectFraction(emptyR, rapid.IntRange(-1000, 1000).Draw(t, "ta")), 0)

		r := genRect(t, "r")
		require.InDelta(t, 0.0, rectFraction(r, rapid.IntRange(-1000, 0).Draw(t, "ta_neg")), 0)
	})
}

func TestProperty_FormatDurationLabel_ContainsStart(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		start := time.Duration(rapid.Int64Range(0, int64(48*time.Hour)).Draw(t, "start"))
		end := time.Duration(rapid.Int64Range(0, int64(48*time.Hour)).Draw(t, "end"))

		got := formatDurationLabel(start, end)
		require.True(t, strings.Contains(got, FormatTimestamp(start)),
			"label %q missing start %v", got, FormatTimestamp(start))
		if start != end {
			require.True(t, strings.Contains(got, FormatTimestamp(end)) && strings.Contains(got, " - "),
				"label %q missing end %v or separator", got, FormatTimestamp(end))
		}
	})
}

func TestProperty_FormatTimestamp_MatchesShape(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		d := time.Duration(rapid.Int64Range(0, int64(100*time.Hour)).Draw(t, "d"))
		got := FormatTimestamp(d)

		require.Contains(t, got, ":")
		require.NotContains(t, got, " ")

		parts := strings.Split(got, ":")
		var hours, mins, secs int
		var err error
		switch len(parts) {
		case 2:
			mins, err = strconv.Atoi(parts[0])
			require.NoError(t, err)
			secs, err = strconv.Atoi(parts[1])
			require.NoError(t, err)

		case 3:
			hours, err = strconv.Atoi(parts[0])
			require.NoError(t, err)
			mins, err = strconv.Atoi(parts[1])
			require.NoError(t, err)
			secs, err = strconv.Atoi(parts[2])
			require.NoError(t, err)

		default:
			t.Fatalf("unexpected timestamp shape: %q", got)
		}

		require.LessOrEqual(t, mins, 59, "mins out of range: %q", got)
		require.LessOrEqual(t, secs, 59, "secs out of range: %q", got)

		expectSec := int(d.Seconds())
		gotSec := hours*3600 + mins*60 + secs
		require.Equal(t, expectSec, gotSec, "round-trip mismatch for d=%v got=%q", d, got)
	})
}

func TestProperty_FormatTimestamp_Monotonic(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		d1 := time.Duration(rapid.Int64Range(0, int64(50*time.Hour)).Draw(t, "d1"))
		extra := time.Duration(rapid.Int64Range(0, int64(50*time.Hour)).Draw(t, "extra"))
		d2 := d1 + extra
		s1 := FormatTimestamp(d1)
		s2 := FormatTimestamp(d2)
		require.LessOrEqual(t, parseFormatted(t, s1), parseFormatted(t, s2),
			"non-monotonic: d1=%v->%q d2=%v->%q", d1, s1, d2, s2)
	})
}

func TestProperty_ExpandCropForTimestamp_WithinScreen(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		screen := genScreenRect(t)
		crop := image.Rect(
			rapid.IntRange(0, screen.Dx()).Draw(t, "crop_x0"),
			rapid.IntRange(0, screen.Dy()).Draw(t, "crop_y0"),
			0, 0,
		)
		crop.Max.X = rapid.IntRange(crop.Min.X, screen.Dx()).Draw(t, "crop_x1")
		crop.Max.Y = rapid.IntRange(crop.Min.Y, screen.Dy()).Draw(t, "crop_y1")

		minWidth := rapid.IntRange(0, screen.Dx()*2).Draw(t, "min_w")
		got := expandCropForTimestamp(crop, minWidth, screen)
		require.True(t, got.In(screen) || got.Empty(),
			"got=%v not within screen=%v crop=%v minW=%d", got, screen, crop, minWidth)
	})
}

func TestProperty_ExpandCropForTimestamp_VerticalUnchanged(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		screen := genScreenRect(t)
		cx0 := rapid.IntRange(0, screen.Dx()-1).Draw(t, "crop_x0")
		cx1 := rapid.IntRange(cx0+1, screen.Dx()).Draw(t, "crop_x1")
		cy0 := rapid.IntRange(0, screen.Dy()-1).Draw(t, "crop_y0")
		cy1 := rapid.IntRange(cy0+1, screen.Dy()).Draw(t, "crop_y1")
		crop := image.Rect(cx0, cy0, cx1, cy1)

		minWidth := rapid.IntRange(0, screen.Dx()*2).Draw(t, "min_w")
		got := expandCropForTimestamp(crop, minWidth, screen)
		require.Equal(t, crop.Min.Y, got.Min.Y, "vertical changed: crop=%v got=%v", crop, got)
		require.Equal(t, crop.Max.Y, got.Max.Y, "vertical changed: crop=%v got=%v", crop, got)
	})
}

func TestProperty_ExpandCropForTimestamp_ReachesMinWidthWhenPossible(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		sw := rapid.IntRange(1, 4096).Draw(t, "screen_w")
		sh := rapid.IntRange(1, 4096).Draw(t, "screen_h")
		screen := image.Rect(0, 0, sw, sh)
		minWidth := rapid.IntRange(0, sw).Draw(t, "min_w")

		cx0 := rapid.IntRange(0, sw-1).Draw(t, "crop_x0")
		cx1 := rapid.IntRange(cx0+1, sw).Draw(t, "crop_x1")
		cy0 := rapid.IntRange(0, sh-1).Draw(t, "crop_y0")
		cy1 := rapid.IntRange(cy0+1, sh).Draw(t, "crop_y1")
		crop := image.Rect(cx0, cy0, cx1, cy1)

		got := expandCropForTimestamp(crop, minWidth, screen)
		require.GreaterOrEqual(t, got.Dx(), minWidth,
			"got width %d < minWidth %d (crop=%v screen=%v)", got.Dx(), minWidth, crop, screen)
	})
}

func TestProperty_FitImageDimensions_BothAtLeastOne(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		w := rapid.IntRange(1, 32_000).Draw(t, "w")
		h := rapid.IntRange(1, 32_000).Draw(t, "h")

		outW, outH := fitImageDimensions(w, h)
		require.GreaterOrEqual(t, outW, 1, "in=%dx%d out=%dx%d", w, h, outW, outH)
		require.GreaterOrEqual(t, outH, 1, "in=%dx%d out=%dx%d", w, h, outW, outH)
	})
}

func TestProperty_FitImageDimensions_RespectsMaxDimension(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		w := rapid.IntRange(1, 32_000).Draw(t, "w")
		h := rapid.IntRange(1, 32_000).Draw(t, "h")

		outW, outH := fitImageDimensions(w, h)
		require.LessOrEqual(t, outW, screenshotMaxDimension,
			"outW=%d > max=%d (in=%dx%d)", outW, screenshotMaxDimension, w, h)
		require.LessOrEqual(t, outH+timestampBarHeight, screenshotMaxDimension,
			"outH+bar=%d > max=%d (in=%dx%d)", outH+timestampBarHeight, screenshotMaxDimension, w, h)
	})
}

func TestProperty_FitImageDimensions_RespectsMaxPixels(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		w := rapid.IntRange(1, 32_000).Draw(t, "w")
		h := rapid.IntRange(1, 32_000).Draw(t, "h")

		outW, outH := fitImageDimensions(w, h)
		totalPixels := outW * (outH + timestampBarHeight)
		require.LessOrEqual(t, totalPixels, screenshotMaxPixels,
			"%d pixels > %d (in=%dx%d out=%dx%d)", totalPixels, screenshotMaxPixels, w, h, outW, outH)
	})
}

func TestProperty_AdaptiveInterval_AtLeastBaseFloor(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		hadActive := rapid.Bool().Draw(t, "had_active")
		taken := rapid.IntRange(0, 100_000).Draw(t, "taken")
		elapsedSec := rapid.Int64Range(0, int64((48*time.Hour).Seconds())).Draw(t, "elapsed_s")
		startTime := time.Unix(1_700_000_000, 0)
		evt := startTime.Add(time.Duration(elapsedSec) * time.Second)

		d := &RecordingProcessor{
			startTime:        startTime,
			hadActiveWindow:  hadActive,
			screenshotsTaken: taken,
		}

		got := d.adaptiveInterval(evt)
		require.GreaterOrEqual(t, got, screenshotMinInterval,
			"interval %v < floor %v (hadActive=%v taken=%d elapsed=%ds)",
			got, screenshotMinInterval, hadActive, taken, elapsedSec)
	})
}

// parseFormatted converts a FormatTimestamp output back to total seconds, used by the monotonicity property.
func parseFormatted(t *rapid.T, s string) int {
	parts := strings.Split(s, ":")
	var a, b, c int
	var err error

	switch len(parts) {
	case 2:
		a, err = strconv.Atoi(parts[0])
		require.NoError(t, err)
		b, err = strconv.Atoi(parts[1])
		require.NoError(t, err)
		return a*60 + b

	case 3:
		a, err = strconv.Atoi(parts[0])
		require.NoError(t, err)
		b, err = strconv.Atoi(parts[1])
		require.NoError(t, err)
		c, err = strconv.Atoi(parts[2])
		require.NoError(t, err)
		return a*3600 + b*60 + c

	default:
		t.Fatalf("unexpected format: %q", s)
		return 0
	}
}

// genRect generates a possibly-non-canonical rectangle. image.Rect canonicalizes  when Min > Max, so this exercises that path too.
func genRect(t *rapid.T, label string) image.Rectangle {
	t.Helper()

	x0 := rapid.IntRange(-1024, 4096).Draw(t, label+"_x0")
	y0 := rapid.IntRange(-1024, 4096).Draw(t, label+"_y0")
	w := rapid.OneOf(
		rapid.Just(0),
		rapid.Just(1),
		rapid.IntRange(1, 4096),
	).Draw(t, label+"_w")
	h := rapid.OneOf(
		rapid.Just(0),
		rapid.Just(1),
		rapid.IntRange(1, 4096),
	).Draw(t, label+"_h")

	return image.Rect(x0, y0, x0+w, y0+h)
}

// genScreenRect generates a non-empty screen rect anchored at (0,0).
func genScreenRect(t *rapid.T) image.Rectangle {
	t.Helper()

	w := rapid.IntRange(1, 4096).Draw(t, "screen_w")
	h := rapid.IntRange(1, 4096).Draw(t, "screen_h")

	return image.Rect(0, 0, w, h)
}
