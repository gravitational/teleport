package desktop

import (
	"image"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWindowTracker_Update(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		regions    []image.Rectangle
		cursor     image.Point
		wantActive image.Rectangle
		wantUnion  image.Rectangle
	}{
		{
			name:       "no regions yields empty rectangles",
			regions:    nil,
			cursor:     image.Pt(0, 0),
			wantActive: image.Rectangle{},
			wantUnion:  image.Rectangle{},
		},
		{
			name:       "single region becomes the active window",
			regions:    []image.Rectangle{image.Rect(10, 10, 50, 50)},
			cursor:     image.Pt(0, 0),
			wantActive: image.Rect(10, 10, 50, 50),
			wantUnion:  image.Rect(10, 10, 50, 50),
		},
		{
			name: "overlapping regions merge into one cluster",
			regions: []image.Rectangle{
				image.Rect(0, 0, 100, 100),
				image.Rect(50, 50, 150, 150),
			},
			cursor:     image.Pt(0, 0),
			wantActive: image.Rect(0, 0, 150, 150),
			wantUnion:  image.Rect(0, 0, 150, 150),
		},
		{
			name: "cursor inside a cluster picks that cluster",
			regions: []image.Rectangle{
				image.Rect(0, 0, 100, 100),
				image.Rect(500, 500, 700, 700),
			},
			cursor:     image.Pt(550, 550),
			wantActive: image.Rect(500, 500, 700, 700),
			wantUnion:  image.Rect(0, 0, 700, 700),
		},
		{
			name: "cursor outside all clusters picks the largest",
			regions: []image.Rectangle{
				image.Rect(0, 0, 10, 10),
				image.Rect(500, 500, 800, 800),
			},
			cursor:     image.Pt(-50, -50),
			wantActive: image.Rect(500, 500, 800, 800),
			wantUnion:  image.Rect(0, 0, 800, 800),
		},
		{
			name: "regions 19px apart merge (within clusterGap)",
			regions: []image.Rectangle{
				image.Rect(0, 0, 10, 10),
				image.Rect(29, 0, 40, 10),
			},
			cursor:     image.Pt(0, 0),
			wantActive: image.Rect(0, 0, 40, 10),
			wantUnion:  image.Rect(0, 0, 40, 10),
		},
		{
			name: "regions exactly 20px apart do not merge",
			regions: []image.Rectangle{
				image.Rect(0, 0, 10, 10),
				image.Rect(30, 0, 60, 30),
			},
			cursor:     image.Pt(-50, -50),
			wantActive: image.Rect(30, 0, 60, 30),
			wantUnion:  image.Rect(0, 0, 60, 30),
		},
		{
			name: "transitive merge across chained nearby regions",
			regions: []image.Rectangle{
				image.Rect(0, 0, 10, 10),
				image.Rect(25, 0, 35, 10),
				image.Rect(50, 0, 60, 10),
			},
			cursor:     image.Pt(5, 5),
			wantActive: image.Rect(0, 0, 60, 10),
			wantUnion:  image.Rect(0, 0, 60, 10),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := newWindowTracker()
			w.Update(tt.regions, tt.cursor)

			require.Equal(t, tt.wantActive, w.ActiveWindow())
			require.Equal(t, tt.wantUnion, w.UnionDirtyRect())
		})
	}
}

func TestWindowTracker_UpdateAccumulates(t *testing.T) {
	t.Parallel()

	w := newWindowTracker()

	w.Update([]image.Rectangle{image.Rect(0, 0, 10, 10)}, image.Pt(0, 0))
	require.Equal(t, image.Rect(0, 0, 10, 10), w.ActiveWindow())
	require.Equal(t, image.Rect(0, 0, 10, 10), w.UnionDirtyRect())

	w.Update([]image.Rectangle{image.Rect(500, 500, 600, 600)}, image.Pt(550, 550))
	require.Equal(t, image.Rect(500, 500, 600, 600), w.ActiveWindow())
	require.Equal(t, image.Rect(0, 0, 600, 600), w.UnionDirtyRect())
}

func TestWindowTracker_Reset(t *testing.T) {
	t.Parallel()

	w := newWindowTracker()
	w.Update([]image.Rectangle{image.Rect(10, 10, 50, 50)}, image.Pt(20, 20))

	require.NotEqual(t, image.Rectangle{}, w.ActiveWindow())
	require.NotEqual(t, image.Rectangle{}, w.UnionDirtyRect())

	w.Reset()

	require.Equal(t, image.Rectangle{}, w.ActiveWindow())
	require.Equal(t, image.Rectangle{}, w.UnionDirtyRect())

	w.Update([]image.Rectangle{image.Rect(100, 100, 200, 200)}, image.Pt(150, 150))
	require.Equal(t, image.Rect(100, 100, 200, 200), w.ActiveWindow())
	require.Equal(t, image.Rect(100, 100, 200, 200), w.UnionDirtyRect())
}
