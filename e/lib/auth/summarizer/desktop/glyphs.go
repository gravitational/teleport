package desktop

import (
	"image"
	"image/color"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	// timestampFontScale is the integer scale factor applied to the basicfont.Face7x13 to produce larger glyphs for
	// timestamp rendering.
	timestampFontScale = 2

	// timestampBarHeight is the height of the black bar at the top of the  screenshot where the timestamp is rendered.
	// This is accounted for in the pixel budget so the final image (screenshot + bar) stays within LLM limits.
	timestampBarHeight = 30
)

// GlyphCache holds pre-rendered glyph images for fast timestamp blitting. Only printable ASCII bytes (32..126) have
// glyphs; all other indices are nil.
type GlyphCache struct {
	glyphs [128]*image.RGBA
	// glyphW and glyphH are the dimensions of each glyph at the rendered scale (basicfont 7x13 * timestampFontScale).
	glyphW int
	glyphH int
}

// NewGlyphCache pre-renders the glyphs needed for timestamp strings (digits, colon, space) at timestampFontScale.
func NewGlyphCache() *GlyphCache {
	const (
		baseW = 7
		baseH = 13
	)
	scaledW := baseW * timestampFontScale
	scaledH := baseH * timestampFontScale

	face := basicfont.Face7x13
	cache := &GlyphCache{
		glyphW: scaledW,
		glyphH: scaledH,
	}

	// Pre-render all printable ASCII characters.
	for c := byte(32); c < 127; c++ {
		small := image.NewRGBA(image.Rect(0, 0, baseW, baseH))
		d := &font.Drawer{
			Dst:  small,
			Src:  image.NewUniform(color.White),
			Face: face,
			Dot:  fixed.P(0, face.Metrics().Ascent.Ceil()),
		}
		d.DrawString(string(c))

		scaled := image.NewRGBA(image.Rect(0, 0, scaledW, scaledH))
		draw.NearestNeighbor.Scale(scaled, scaled.Bounds(), small, small.Bounds(), draw.Over, nil)
		cache.glyphs[c] = scaled
	}

	return cache
}

// stampLabel draws the given label string onto the provided image using the pre-rendered glyphs in the cache.
// The label is centered horizontally within the timestamp bar at the top of the image. label must contain only
// single-byte (ASCII) characters; non-ASCII bytes and non-printable bytes are skipped (their slot is left blank).
func (g *GlyphCache) stampLabel(img *image.RGBA, label string) {
	textW := len(label) * g.glyphW
	barW := img.Bounds().Dx()
	x := (barW - textW) / 2
	y := (timestampBarHeight - g.glyphH) / 2

	for i := 0; i < len(label); i++ {
		b := label[i]
		var glyph *image.RGBA
		if int(b) < len(g.glyphs) {
			glyph = g.glyphs[b]
		}
		if glyph == nil {
			x += g.glyphW
			continue
		}

		dst := image.Rect(x, y, x+g.glyphW, y+g.glyphH)
		draw.Draw(img, dst, glyph, image.Point{}, draw.Over)

		x += g.glyphW
	}
}
