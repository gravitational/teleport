package desktop

import "image"

// clusterGap is the pixel distance within which separate update regions are merged into a single cluster.
// Two rectangles whose nearest edges are fewer than clusterGap pixels apart are merged; rectangles exactly
// clusterGap pixels apart remain separate.
const clusterGap = 20

// windowTracker clusters RDP update regions to detect the active window.
// It uses cursor position as the primary signal: the cluster containing the cursor is active; when the cursor sits
// outside every cluster, the largest one wins.
//
// Clustering is performed incrementally: each Update merges the new regions into the existing clusters rather than
// re-clustering the full history, keeping per-update cost proportional to the number of distinct clusters on screen
// instead of growing unbounded with session length.
type windowTracker struct {
	clusters     []image.Rectangle
	merged       []bool // backing array reused by clusterRegions across calls
	cursor       image.Point
	activeWindow image.Rectangle
	unionDirty   image.Rectangle
	hasUpdates   bool
}

func newWindowTracker() *windowTracker {
	return &windowTracker{}
}

// Update feeds update regions and cursor position, then recomputes the active window and union dirty rectangle.
func (w *windowTracker) Update(regions []image.Rectangle, cursor image.Point) {
	w.cursor = cursor

	for _, r := range regions {
		if w.hasUpdates {
			w.unionDirty = w.unionDirty.Union(r)
		} else {
			w.unionDirty = r
			w.hasUpdates = true
		}
	}

	w.clusters = append(w.clusters, regions...)
	w.clusters, w.merged = clusterRegions(w.clusters, w.merged, clusterGap)

	if len(w.clusters) == 0 {
		w.activeWindow = image.Rectangle{}
		return
	}

	w.activeWindow = pickActiveCluster(w.clusters, w.cursor)
}

// ActiveWindow returns the current active window as determined by the tracker, or an empty rectangle if no active window is detected.
func (w *windowTracker) ActiveWindow() image.Rectangle {
	return w.activeWindow
}

// UnionDirtyRect returns the bounding rectangle that encompasses all update regions fed to the tracker since the last
// reset, or an empty rectangle if no updates have been fed.
func (w *windowTracker) UnionDirtyRect() image.Rectangle {
	return w.unionDirty
}

// Reset clears accumulated state while keeping the clusters and merged slices' backing arrays for reuse.
func (w *windowTracker) Reset() {
	w.clusters = w.clusters[:0]
	w.merged = w.merged[:0]
	w.activeWindow = image.Rectangle{}
	w.unionDirty = image.Rectangle{}
	w.hasUpdates = false
}

// clusterRegions merges spatially overlapping or nearby rectangles in regions in place. Two rectangles whose nearest
// edges are fewer than gap pixels apart are merged; rectangles exactly gap pixels apart remain separate. The returned
// slice is a prefix of regions containing the surviving clusters.
func clusterRegions(regions []image.Rectangle, merged []bool, gap int) ([]image.Rectangle, []bool) {
	if len(regions) == 0 {
		return regions[:0], merged[:0]
	}

	if cap(merged) < len(regions) {
		merged = make([]bool, len(regions))
	} else {
		merged = merged[:len(regions)]
		clear(merged)
	}

	changed := true
	for changed {
		changed = false

		for i := 0; i < len(regions); i++ {
			if merged[i] {
				continue
			}

			for j := i + 1; j < len(regions); j++ {
				if merged[j] {
					continue
				}

				if regions[i].Inset(-gap).Overlaps(regions[j]) {
					regions[i] = regions[i].Union(regions[j])
					merged[j] = true
					changed = true
				}
			}
		}
	}

	// Compact surviving clusters to the front of regions.
	w := 0
	for i := 0; i < len(regions); i++ {
		if merged[i] {
			continue
		}

		regions[w] = regions[i]
		w++
	}

	return regions[:w], merged
}

// pickActiveCluster returns the cluster containing the cursor, or the largest cluster if the cursor is outside all clusters.
func pickActiveCluster(clusters []image.Rectangle, cursor image.Point) image.Rectangle {
	for _, c := range clusters {
		if cursor.In(c) {
			return c
		}
	}

	largest := clusters[0]
	largestArea := largest.Dx() * largest.Dy()

	for _, c := range clusters[1:] {
		area := c.Dx() * c.Dy()
		if area > largestArea {
			largest = c
			largestArea = area
		}
	}

	return largest
}
