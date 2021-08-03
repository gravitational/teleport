package deskproto

import (
	"image"
	"image/color"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecode(t *testing.T) {
	for _, m := range []Message{
		MouseMove{X: 1, Y: 2},
		MouseButton{Button: MiddleMouseButton, State: ButtonPressed},
		KeyboardButton{KeyCode: 1, State: ButtonPressed},
		func() Message {
			img := image.NewNRGBA(image.Rect(5, 5, 10, 10))
			for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
				for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
					img.Set(x, y, color.NRGBA{1, 2, 3, 4})
				}
			}
			return PNGFrame{Img: img}
		}(),
	} {

		buf, err := m.Encode()
		require.NoError(t, err)

		out, err := Decode(buf)
		require.NoError(t, err)

		require.Empty(t, cmp.Diff(m, out))
	}
}

func TestBadDecode(t *testing.T) {
	// 254 is an unknown message type.
	_, err := Decode([]byte{254})
	require.Error(t, err)
}
