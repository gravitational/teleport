package desktop

import (
	"image"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewGlyphCache(t *testing.T) {
	t.Parallel()

	cache := NewGlyphCache()

	require.Equal(t, 7*timestampFontScale, cache.glyphW)
	require.Equal(t, 13*timestampFontScale, cache.glyphH)

	for c := byte(32); c < 127; c++ {
		glyph := cache.glyphs[c]
		require.NotNil(t, glyph, "missing glyph for byte %d (%q)", c, c)

		require.Equal(t, cache.glyphW, glyph.Bounds().Dx())
		require.Equal(t, cache.glyphH, glyph.Bounds().Dy())
	}

	require.Nil(t, cache.glyphs[0], "non-printable bytes should not be cached")
	require.Nil(t, cache.glyphs[127], "non-printable bytes should not be cached")
}

func TestGlyphCache_StampLabel(t *testing.T) {
	t.Parallel()

	cache := NewGlyphCache()

	t.Run("draws centered label inside the timestamp bar", func(t *testing.T) {
		t.Parallel()

		const (
			imgW  = 200
			label = "12:34"
		)
		img := image.NewRGBA(image.Rect(0, 0, imgW, timestampBarHeight))

		cache.stampLabel(img, label)

		textW := len(label) * cache.glyphW
		wantX := (imgW - textW) / 2
		wantY := (timestampBarHeight - cache.glyphH) / 2
		drawn := image.Rect(wantX, wantY, wantX+textW, wantY+cache.glyphH)

		require.True(t, hasNonZeroPixel(img, drawn),
			"expected drawn pixels inside %v", drawn)

		leftMargin := image.Rect(0, 0, wantX, timestampBarHeight)
		rightMargin := image.Rect(wantX+textW, 0, imgW, timestampBarHeight)
		require.False(t, hasNonZeroPixel(img, leftMargin), "expected blank left margin")
		require.False(t, hasNonZeroPixel(img, rightMargin), "expected blank right margin")
	})

	t.Run("skips non-printable bytes without panicking", func(t *testing.T) {
		t.Parallel()

		img := image.NewRGBA(image.Rect(0, 0, 200, timestampBarHeight))

		require.NotPanics(t, func() {
			cache.stampLabel(img, string([]byte{0x01, '1', 0xFF, '2'}))
		})
	})

	t.Run("label wider than image still draws without panic", func(t *testing.T) {
		t.Parallel()

		img := image.NewRGBA(image.Rect(0, 0, 10, timestampBarHeight))

		require.NotPanics(t, func() {
			cache.stampLabel(img, "12345678")
		})
	})

	t.Run("empty label leaves the image untouched", func(t *testing.T) {
		t.Parallel()

		img := image.NewRGBA(image.Rect(0, 0, 200, timestampBarHeight))

		cache.stampLabel(img, "")

		require.False(t, hasNonZeroPixel(img, img.Bounds()), "expected no pixels drawn")
	})
}

// hasNonZeroPixel reports whether any pixel inside r has a non-zero RGBA value.
func hasNonZeroPixel(img *image.RGBA, r image.Rectangle) bool {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0 {
				return true
			}
		}
	}

	return false
}
