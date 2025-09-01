/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package recordingmetadatav1

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
)

const (
	nearCursorRadiusPx = 256 // small-block zone radius around cursor
	smallBlockSize     = 16  // fine granularity near cursor
	largeBlockSize     = 32  // default elsewhere
)

var defaultKeyframePolicy = keyframePolicy{
	maxSeconds:    3.0,  // max time between keyframes
	maxDeltas:     45,   // max chain length
	areaThreshold: 0.33, // if changed area ≥ 33%, force KF
}

type keyframePolicy struct {
	maxSeconds    float64 // 2–3
	maxDeltas     int     // 30–45
	areaThreshold float64 // 0.30–0.40
}

// decideKeyframe returns true if we should keyframe now.
func decideKeyframe(secondsSinceKF float64, deltasSinceKF int, changedAreaFrac float64, p keyframePolicy) bool {
	if changedAreaFrac >= p.areaThreshold {
		return true
	}
	if secondsSinceKF >= p.maxSeconds {
		return true
	}
	if deltasSinceKF >= p.maxDeltas {
		return true
	}
	return false
}

type frameSnapshot struct {
	timestamp time.Duration
	cursorX   int
	cursorY   int
	frameRef  uint32
}

type deltaFrame struct {
	baseRef uint32
	patches []framePatch // raw pixel patches
}

type framePatch struct {
	rect image.Rectangle
	img  *image.RGBA // raw pixels for this rect
}

type frameData struct {
	image  *image.RGBA
	width  int
	height int
}

type desktopThumbnailCapture struct {
	// circular buffer of snapshots
	snapshots        []frameSnapshot
	frameIndex       uint32
	currentFrame     *image.RGBA
	mu               sync.RWMutex
	interval         time.Duration
	keyframes        map[uint32]*frameData
	deltaFrames      map[uint32]*deltaFrame
	lastSnapshotTime time.Duration
	frameChanged     bool
	kfPolicy         keyframePolicy

	framesSinceKeyframe int
	currentFrameRef     uint32
	lastKeyframe        uint32
	lastKeyframeTime    time.Duration
	lastDecodedFrame    *image.RGBA // last materialized frame (raw)

	cursorX           atomic.Int32
	cursorY           atomic.Int32
	cursorPositionSet atomic.Bool // Track if cursor position has been explicitly set

	// Cursor bitmap data
	cursorBitmap   *image.RGBA
	cursorHotspotX int
	cursorHotspotY int
	cursorVisible  bool

	// dirty regions accumulated since last snapshot
	dirtyRects []image.Rectangle

	// worker semaphore for patch building
	workers int

	// RDP decoder for processing Fast Path PDUs
	rdpDecoder *RDPDecoder
}

func newDesktopThumbnailCapture(interval time.Duration, size int) (*desktopThumbnailCapture, error) {
	d := &desktopThumbnailCapture{
		snapshots:   make([]frameSnapshot, 0, size),
		keyframes:   make(map[uint32]*frameData),
		deltaFrames: make(map[uint32]*deltaFrame),
		interval:    interval,
		workers:     runtime.GOMAXPROCS(0),
		kfPolicy:    defaultKeyframePolicy,
		// RDP decoder will be created lazily when we know the actual dimensions
		rdpDecoder: nil,
		// Default cursor to visible (will be hidden if we get a hide event)
		cursorVisible: true,
	}
	// cursorPositionSet defaults to false, positions default to 0,0
	// We'll show cursor at center if no position updates are received
	return d, nil
}

func (d *desktopThumbnailCapture) handleEvent(event *apievents.DesktopRecording) error {
	msg, err := tdp.Decode(event.Message)
	if err != nil {
		return trace.Wrap(err, "failed to decode desktop recording event")
	}

	switch m := msg.(type) {
	case tdp.RDPFastPathPDU:
		// Process RDP Fast Path PDU through IronRDP decoder
		// Create decoder lazily if not initialized
		if d.rdpDecoder == nil {
			// Try to detect dimensions from the PDU or use safe defaults
			// We'll start with a small decoder that can resize itself
			decoder, err := NewRDPDecoder(0, 0)
			if err != nil {
				return trace.Wrap(err, "failed to create RDP decoder")
			}
			d.rdpDecoder = decoder
		}

		decoderOutput, err := d.rdpDecoder.ProcessFastPathPDU([]byte(m))
		if err != nil {
			return trace.Wrap(err, "failed to process Fast Path PDU")
		}
		if decoderOutput != nil {
			// Handle frame updates
			if decoderOutput.FrameUpdate != nil {
				d.applyFrameUpdateFromRDP(decoderOutput.FrameUpdate)
				d.frameChanged = true
				// mark dirty
				d.mu.Lock()
				d.dirtyRects = append(d.dirtyRects, decoderOutput.FrameUpdate.Image.Bounds())
				d.mu.Unlock()
			}

			// Handle pointer updates
			if decoderOutput.PointerUpdate != nil {
				d.updateCursorBitmap(decoderOutput.PointerUpdate)
			}

			// Handle pointer position updates
			if decoderOutput.PointerPosition != nil {
				d.updateCursorPosition(int(decoderOutput.PointerPosition.X), int(decoderOutput.PointerPosition.Y))
			}

			// Handle pointer visibility
			if decoderOutput.PointerHidden {
				d.mu.Lock()
				d.cursorVisible = false
				d.mu.Unlock()
			} else {
				d.mu.Lock()
				d.cursorVisible = true
				// Could set a default cursor here if needed
				d.mu.Unlock()
			}
		}
	case tdp.PNGFrame:
		d.applyFrameUpdate(&m)
		d.frameChanged = true
		// mark dirty
		if m.Img != nil {
			d.mu.Lock()
			d.dirtyRects = append(d.dirtyRects, m.Img.Bounds())
			d.mu.Unlock()
		}
	case tdp.MouseMove:
		d.updateCursorPosition(int(m.X), int(m.Y))
	}

	ms := time.Duration(event.DelayMilliseconds) * time.Millisecond

	if ms-d.lastSnapshotTime >= d.interval {
		d.lastSnapshotTime = ms

		if d.frameChanged {
			d.framesSinceKeyframe++

			// Analyze changes once (merged rects + area fraction), then decide KF vs delta
			var mergedRects []image.Rectangle
			var changedFrac float64
			var hadChanges bool

			d.mu.Lock()
			cur := d.currentFrame
			lastImg := d.lastDecodedFrame
			dirty := append([]image.Rectangle(nil), d.dirtyRects...)
			d.mu.Unlock()

			if cur != nil && lastImg != nil && len(dirty) > 0 {
				mergedRects = d.computeMergedChangeRects(cur, lastImg, dirty)
				if len(mergedRects) > 0 {
					hadChanges = true
					changedFrac = areaFraction(mergedRects, cur.Bounds())
				}
			}

			// If no changes detected vs last decoded: keep reference the same
			if !hadChanges && d.lastKeyframe != 0 {
				// No new delta or keyframe; just snapshot the same frameRef
				d.currentFrameRef = d.lastKeyframe // safe default
			} else {
				// Decide whether to keyframe based on policy
				secondsSinceKF := 0.0
				if d.lastKeyframeTime > 0 {
					secondsSinceKF = (ms - d.lastKeyframeTime).Seconds()
				}
				shouldKF := (d.lastKeyframe == 0) ||
					decideKeyframe(secondsSinceKF, d.framesSinceKeyframe, changedFrac, d.kfPolicy)

				if shouldKF {
					keyframeRef, err := d.storeKeyframeLocked(ms)
					if err != nil {
						return trace.Wrap(err, "failed to store keyframe")
					}
					d.currentFrameRef = keyframeRef
					d.framesSinceKeyframe = 0
				} else {
					// Build delta from the already-computed mergedRects
					deltaRef, err := d.storeDeltaFromRects(cur, mergedRects)
					if err != nil {
						return trace.Wrap(err, "failed to store delta frame")
					}
					d.currentFrameRef = deltaRef
				}
			}

			d.frameChanged = false

			// reset dirty
			d.mu.Lock()
			d.dirtyRects = d.dirtyRects[:0]
			d.mu.Unlock()
		}

		d.captureSnapshot(ms, d.currentFrameRef)
	}

	return nil
}

// computeMergedChangeRects returns merged rects of blocks that differ between cur and prev,
// scanning only within the dirty regions and using a hybrid grid (small near cursor, large elsewhere).
func (d *desktopThumbnailCapture) computeMergedChangeRects(cur, prev *image.RGBA, dirty []image.Rectangle) []image.Rectangle {
	if cur == nil || prev == nil || len(dirty) == 0 {
		return nil
	}
	bounds := cur.Bounds()
	cx := clamp(int(d.cursorX.Load()), 0, bounds.Dx()-1)
	cy := clamp(int(d.cursorY.Load()), 0, bounds.Dy()-1)

	near := image.Rect(
		max(0, cx-nearCursorRadiusPx),
		max(0, cy-nearCursorRadiusPx),
		min(bounds.Max.X, cx+nearCursorRadiusPx),
		min(bounds.Max.Y, cy+nearCursorRadiusPx),
	)

	seen := make(map[uint64]struct{}, 256)
	var rects []image.Rectangle

	// Pass 1: small blocks inside "near"
	for _, dr := range dirty {
		r := dr.Intersect(bounds).Intersect(near)
		if r.Empty() {
			continue
		}
		iterateBlocks(r, bounds, smallBlockSize, func(br image.Rectangle) {
			if blocksDiffer(cur, prev, br) {
				addUniqueRect(&rects, seen, br)
			}
		})
	}

	// Pass 2: large blocks outside "near"
	for _, dr := range dirty {
		r := dr.Intersect(bounds)
		if r.Empty() {
			continue
		}
		iterateBlocks(r, bounds, largeBlockSize, func(br image.Rectangle) {
			if br.Overlaps(near) {
				return // small-block pass handled this area
			}
			if blocksDiffer(cur, prev, br) {
				addUniqueRect(&rects, seen, br)
			}
		})
	}

	return mergeRectangles(rects)
}

func (d *desktopThumbnailCapture) applyFrameUpdate(frame *tdp.PNGFrame) {
	if frame.Img == nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	bounds := frame.Img.Bounds()

	if d.currentFrame == nil {
		// Initialize canvas. If first frame is offset, size to Max.X/Max.Y
		canvas := image.Rect(0, 0, bounds.Max.X, bounds.Max.Y)
		if bounds.Min.X == 0 && bounds.Min.Y == 0 {
			canvas = image.Rect(0, 0, bounds.Dx(), bounds.Dy())
		}
		d.currentFrame = image.NewRGBA(canvas)
	}

	// Expand canvas if needed (grow by powers of two to reduce realloc frequency)
	c := d.currentFrame.Bounds()
	newW, newH := c.Dx(), c.Dy()
	needGrow := false
	if bounds.Max.X > c.Max.X {
		newW = growPow2(c.Max.X, bounds.Max.X)
		needGrow = true
	}
	if bounds.Max.Y > c.Max.Y {
		newH = growPow2(c.Max.Y, bounds.Max.Y)
		needGrow = true
	}
	if needGrow {
		newFrame := image.NewRGBA(image.Rect(0, 0, newW, newH))
		draw.Draw(newFrame, c, d.currentFrame, image.Point{}, draw.Src)
		d.currentFrame = newFrame
	}

	// Blit patch
	draw.Draw(d.currentFrame, bounds, frame.Img, bounds.Min, draw.Src)
}

// applyFrameUpdateFromRDP applies an RDP-decoded frame update to the current frame
func (d *desktopThumbnailCapture) applyFrameUpdateFromRDP(update *FrameUpdate) {
	if update == nil || update.Image == nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	bounds := update.Image.Bounds()

	if d.currentFrame == nil {
		// Initialize canvas. If first frame is offset, size to Max.X/Max.Y
		canvas := image.Rect(0, 0, bounds.Max.X, bounds.Max.Y)
		if bounds.Min.X == 0 && bounds.Min.Y == 0 {
			canvas = image.Rect(0, 0, bounds.Dx(), bounds.Dy())
		}
		d.currentFrame = image.NewRGBA(canvas)
	}

	// Expand canvas if needed (grow by powers of two to reduce realloc frequency)
	c := d.currentFrame.Bounds()
	newW, newH := c.Dx(), c.Dy()
	needGrow := false
	if bounds.Max.X > c.Max.X {
		newW = growPow2(c.Max.X, bounds.Max.X)
		needGrow = true
	}
	if bounds.Max.Y > c.Max.Y {
		newH = growPow2(c.Max.Y, bounds.Max.Y)
		needGrow = true
	}
	if needGrow {
		newFrame := image.NewRGBA(image.Rect(0, 0, newW, newH))
		draw.Draw(newFrame, c, d.currentFrame, image.Point{}, draw.Src)
		d.currentFrame = newFrame
	}

	// Blit patch
	draw.Draw(d.currentFrame, bounds, update.Image, bounds.Min, draw.Src)
}

func growPow2(old, need int) int {
	if old == 0 {
		old = 1
	}
	n := old
	for n < need {
		n <<= 1
	}
	return n
}

func areaFraction(rects []image.Rectangle, bounds image.Rectangle) float64 {
	if len(rects) == 0 {
		return 0
	}
	total := bounds.Dx() * bounds.Dy()
	if total <= 0 {
		return 0
	}
	changed := 0
	for _, r := range rects {
		changed += r.Dx() * r.Dy()
	}
	return float64(changed) / float64(total)
}

func (d *desktopThumbnailCapture) storeKeyframeLocked(now time.Duration) (uint32, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.currentFrame == nil {
		return 0, nil
	}

	clone := cloneRGBA(d.currentFrame)

	d.frameIndex++
	d.keyframes[d.frameIndex] = &frameData{
		image:  clone,
		width:  clone.Bounds().Dx(),
		height: clone.Bounds().Dy(),
	}
	d.lastKeyframe = d.frameIndex
	d.lastKeyframeTime = now
	d.lastDecodedFrame = clone

	return d.frameIndex, nil
}

func cloneRGBA(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	copy(dst.Pix, src.Pix)
	return dst
}

func (d *desktopThumbnailCapture) storeDeltaFromRects(cur *image.RGBA, merged []image.Rectangle) (uint32, error) {
	if cur == nil || len(merged) == 0 {
		return d.lastKeyframe, nil
	}

	// Build patches (copy pixels once per merged rect)
	patches := make([]framePatch, len(merged))

	type job struct {
		rect image.Rectangle
		idx  int
	}
	jobs := make(chan job, len(merged))
	wg := sync.WaitGroup{}
	workers := min(d.workers, len(merged))
	if workers < 1 {
		workers = 1
	}
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				sub := cropCopyRGBA(cur, j.rect) // tight copy
				patches[j.idx] = framePatch{rect: j.rect, img: sub}
			}
		}()
	}
	for i, r := range merged {
		jobs <- job{rect: r, idx: i}
	}
	close(jobs)
	wg.Wait()

	// Commit delta frame
	d.mu.Lock()
	d.frameIndex++
	ref := d.frameIndex
	d.deltaFrames[ref] = &deltaFrame{
		baseRef: d.lastKeyframe,
		patches: patches,
	}
	// Update lastDecodedFrame to current for next diff
	d.lastDecodedFrame = cloneRGBA(cur)
	d.mu.Unlock()

	return ref, nil
}

// It only scans inside the provided dirty regions and skips overlap between passes.
func (d *desktopThumbnailCapture) changedBlocksHybrid(
	cur, prev *image.RGBA,
	dirty []image.Rectangle,
	radiusPx, smallBS, largeBS int,
) []image.Rectangle {
	if cur == nil || prev == nil || len(dirty) == 0 {
		return nil
	}

	bounds := cur.Bounds()
	// cursor position (clamped)
	cx := clamp(int(d.cursorX.Load()), 0, bounds.Dx()-1)
	cy := clamp(int(d.cursorY.Load()), 0, bounds.Dy()-1)

	// Square around cursor (fast to compute and iterate)
	near := image.Rect(
		max(0, cx-radiusPx),
		max(0, cy-radiusPx),
		min(bounds.Max.X, cx+radiusPx),
		min(bounds.Max.Y, cy+radiusPx),
	)

	// Collect block rects without duplicates
	// Key: x,y,w,h encoded as string
	seen := make(map[uint64]struct{}, 256)
	out := make([]image.Rectangle, 0, 256)

	// Pass 1: small blocks inside "near"
	for _, dr := range dirty {
		r := dr.Intersect(bounds).Intersect(near)
		if r.Empty() {
			continue
		}
		iterateBlocks(r, bounds, smallBS, func(br image.Rectangle) {
			if blocksDiffer(cur, prev, br) {
				if addUniqueRect(&out, seen, br) {
					// appended
				}
			}
		})
	}

	// Pass 2: large blocks in dirty but skipping any overlap with "near" square
	for _, dr := range dirty {
		r := dr.Intersect(bounds)
		if r.Empty() {
			continue
		}
		iterateBlocks(r, bounds, largeBS, func(br image.Rectangle) {
			if br.Overlaps(near) {
				// this area was (or will be) covered by small blocks — skip
				return
			}
			if blocksDiffer(cur, prev, br) {
				addUniqueRect(&out, seen, br)
			}
		})
	}

	// Coalesce neighboring/overlapping rects
	return mergeRectangles(out)
}

// iterateBlocks calls fn for each block-aligned rectangle covering 'scan'.
// Blocks are aligned to the given blockSize grid within overall 'bounds'.
func iterateBlocks(scan, bounds image.Rectangle, blockSize int, fn func(image.Rectangle)) {
	if blockSize <= 0 {
		blockSize = 32
	}
	startBlockY := (scan.Min.Y / blockSize) * blockSize
	endBlockY := ((scan.Max.Y + blockSize - 1) / blockSize) * blockSize
	startBlockX := (scan.Min.X / blockSize) * blockSize
	endBlockX := ((scan.Max.X + blockSize - 1) / blockSize) * blockSize

	for y := startBlockY; y < endBlockY; y += blockSize {
		for x := startBlockX; x < endBlockX; x += blockSize {
			br := image.Rect(
				x,
				y,
				min(x+blockSize, bounds.Max.X),
				min(y+blockSize, bounds.Max.Y),
			)
			if !br.Overlaps(scan) {
				continue
			}
			fn(br)
		}
	}
}

// addUniqueRect inserts rect iff it hasn't been seen; returns true if added.
func addUniqueRect(out *[]image.Rectangle, seen map[uint64]struct{}, r image.Rectangle) bool {
	// Pack x,y,w,h into a uint64 key to avoid allocations. Assumes 16-bit per field (fits up to 65535).
	// If you might exceed that, switch to a string key or widen the packing.
	w := r.Dx()
	h := r.Dy()
	key := (uint64(r.Min.X)&0xFFFF)<<48 |
		(uint64(r.Min.Y)&0xFFFF)<<32 |
		(uint64(w)&0xFFFF)<<16 |
		(uint64(h) & 0xFFFF)
	if _, ok := seen[key]; ok {
		return false
	}
	seen[key] = struct{}{}
	*out = append(*out, r)
	return true
}

func blocksDiffer(img1, img2 *image.RGBA, rect image.Rectangle) bool {
	if img1 == nil || img2 == nil {
		return true
	}

	// Intersect the rect with both image bounds to ensure we're within valid range
	r1 := rect.Intersect(img1.Bounds())
	r2 := rect.Intersect(img2.Bounds())

	// If the intersections don't match or are empty, the blocks differ
	if r1.Empty() || r2.Empty() || r1 != r2 {
		return true
	}

	// Use the intersected rectangle for comparison
	rect = r1
	w := rect.Dx()

	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		// Ensure we're within bounds for both images
		if y < 0 || y >= img1.Bounds().Max.Y || y >= img2.Bounds().Max.Y {
			return true
		}

		o1 := y*img1.Stride + rect.Min.X*4
		o2 := y*img2.Stride + rect.Min.X*4

		// Additional bounds check to prevent slice out of bounds
		if o1+w*4 > len(img1.Pix) || o2+w*4 > len(img2.Pix) {
			return true
		}

		row1 := img1.Pix[o1 : o1+w*4]
		row2 := img2.Pix[o2 : o2+w*4]

		// bytes.Equal is highly optimized in Go
		if !bytes.Equal(row1, row2) {
			return true
		}
	}
	return false
}

// rectsMergeable returns true if rectangles overlap or share an edge (not just a corner).
func rectsMergeable(a, b image.Rectangle) bool {
	// Horizontal adjacency with vertical overlap
	if a.Max.X == b.Min.X || b.Max.X == a.Min.X {
		return a.Min.Y < b.Max.Y && a.Max.Y > b.Min.Y
	}
	// Vertical adjacency with horizontal overlap
	if a.Max.Y == b.Min.Y || b.Max.Y == a.Min.Y {
		return a.Min.X < b.Max.X && a.Max.X > b.Min.X
	}
	// Overlap
	return a.Overlaps(b)
}

// mergeRectangles coalesces overlapping or edge-touching rects until stable.
func mergeRectangles(rects []image.Rectangle) []image.Rectangle {
	if len(rects) <= 1 {
		return rects
	}
	// Work on a copy
	out := append([]image.Rectangle(nil), rects...)
	for {
		changed := false
		for i := 0; i < len(out); i++ {
			for j := i + 1; j < len(out); j++ {
				if rectsMergeable(out[i], out[j]) {
					out[i] = out[i].Union(out[j])
					// remove j
					out = append(out[:j], out[j+1:]...)
					changed = true
					j-- // adjust index after removal
				}
			}
		}
		if !changed {
			break
		}
	}
	return out
}

// cropCopyRGBA returns a tight *image.RGBA* copy of rect from src.
func cropCopyRGBA(src *image.RGBA, rect image.Rectangle) *image.RGBA {
	r := rect.Intersect(src.Bounds())
	dst := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	// copy rows
	for y := 0; y < r.Dy(); y++ {
		sOff := (r.Min.Y+y)*src.Stride + r.Min.X*4
		dOff := y*dst.Stride + 0
		copy(dst.Pix[dOff:dOff+r.Dx()*4], src.Pix[sOff:sOff+r.Dx()*4])
	}
	return dst
}

func (d *desktopThumbnailCapture) updateCursorPosition(x, y int) {
	d.cursorX.Store(int32(x))
	d.cursorY.Store(int32(y))
	d.cursorPositionSet.Store(true)
}

// updateCursorBitmap updates the stored cursor bitmap from a pointer update
func (d *desktopThumbnailCapture) updateCursorBitmap(update *PointerUpdate) {
	if update == nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Create RGBA image from the cursor data
	rect := image.Rect(0, 0, int(update.Width), int(update.Height))
	cursor := &image.RGBA{
		Pix:    update.Data,
		Stride: int(update.Width) * 4,
		Rect:   rect,
	}

	d.cursorBitmap = cursor
	d.cursorHotspotX = int(update.HotspotX)
	d.cursorHotspotY = int(update.HotspotY)
	d.cursorVisible = true
}

func (d *desktopThumbnailCapture) captureSnapshot(ts time.Duration, frameRef uint32) {
	s := frameSnapshot{
		timestamp: ts,
		cursorX:   int(d.cursorX.Load()),
		cursorY:   int(d.cursorY.Load()),
		frameRef:  frameRef,
	}
	d.mu.Lock()
	d.snapshots = append(d.snapshots, s)
	d.mu.Unlock()
}

// Reconstructs a frameRef into a raw image (no WebP anywhere here)
func (d *desktopThumbnailCapture) reconstructFrame(frameRef uint32) (image.Image, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	visited := make(map[uint32]bool)
	return d.reconstructFrameInternal(frameRef, visited)
}

func (d *desktopThumbnailCapture) reconstructFrameInternal(frameRef uint32, visited map[uint32]bool) (image.Image, error) {
	if visited[frameRef] {
		return nil, fmt.Errorf("circular reference detected")
	}
	visited[frameRef] = true

	if kf, ok := d.keyframes[frameRef]; ok && kf.image != nil {
		return kf.image, nil
	}

	if df, ok := d.deltaFrames[frameRef]; ok {
		base, err := d.reconstructFrameInternal(df.baseRef, visited)
		if err != nil {
			return nil, err
		}
		baseRGBA, ok := base.(*image.RGBA)
		if !ok {
			tmp := image.NewRGBA(base.Bounds())
			draw.Draw(tmp, tmp.Bounds(), base, image.Point{}, draw.Src)
			baseRGBA = tmp
		}
		result := cloneRGBA(baseRGBA)
		for _, p := range df.patches {
			draw.Draw(result, p.rect, p.img, image.Point{}, draw.Src)
		}
		return result, nil
	}

	return nil, fmt.Errorf("frame %d not found", frameRef)
}

type thumbnailOptions struct {
	width      int
	height     int
	zoomFactor float64 // 1.0 = no zoom; >1 zoom-in around cursor
}

// ONLY encodes to WebP here at the very end.
func (d *desktopThumbnailCapture) GenerateThumbnail(index int, opts thumbnailOptions) ([]byte, error) {
	d.mu.RLock()
	if index >= len(d.snapshots) {
		d.mu.RUnlock()
		return nil, nil
	}
	snap := d.snapshots[index]
	d.mu.RUnlock()

	if snap.frameRef == 0 {
		return nil, nil
	}

	img, err := d.reconstructFrame(snap.frameRef)
	if err != nil {
		return nil, err
	}
	rgba, ok := img.(*image.RGBA)
	if !ok {
		tmp := image.NewRGBA(img.Bounds())
		draw.Draw(tmp, tmp.Bounds(), img, image.Point{}, draw.Src)
		rgba = tmp
	}

	// Find the actual content bounds (non-transparent area)
	// The canvas might be larger than the actual desktop content due to power-of-2 growing
	contentBounds := findContentBounds(rgba)
	var processedImg image.Image = rgba
	if !contentBounds.Empty() {
		// Crop to actual content to remove empty padding
		processedImg = imaging.Crop(rgba, contentBounds)
	}

	// Always zoom in 2x at cursor position for thumbnails
	var sourceImg image.Image = processedImg
	
	// Always apply 2x zoom at cursor position
	zoomFactor := 2.0
	b := processedImg.Bounds()
	zoomedW := int(float64(b.Dx()) / zoomFactor)
	zoomedH := int(float64(b.Dy()) / zoomFactor)
	if zoomedW < 1 {
		zoomedW = 1
	}
	if zoomedH < 1 {
		zoomedH = 1
	}

	// Adjust cursor position relative to content bounds
	adjustedCursorX := snap.cursorX
	adjustedCursorY := snap.cursorY
	if !contentBounds.Empty() {
		adjustedCursorX = snap.cursorX - contentBounds.Min.X
		adjustedCursorY = snap.cursorY - contentBounds.Min.Y
	}

	// Center the zoom area around the cursor
	cx := clamp(adjustedCursorX-zoomedW/2, 0, b.Dx()-zoomedW)
	cy := clamp(adjustedCursorY-zoomedH/2, 0, b.Dy()-zoomedH)
	crop := image.Rect(cx, cy, cx+zoomedW, cy+zoomedH)
	sourceImg = imaging.Crop(processedImg, crop)

	// IMPROVED: Use smart scaling for better quality
	// Calculate the optimal scaling approach
	sourceBounds := sourceImg.Bounds()
	sourceW := sourceBounds.Dx()
	sourceH := sourceBounds.Dy()

	// Determine if we're downscaling significantly
	scaleFactorW := float64(sourceW) / float64(opts.width)
	scaleFactorH := float64(sourceH) / float64(opts.height)
	maxScaleFactor := max(scaleFactorW, scaleFactorH)

	var thumb image.Image

	if maxScaleFactor > 3.0 {
		// For very significant downscaling (>3x), use three-step approach
		// This provides the best quality for large reductions
		intermediateW1 := opts.width * 4
		intermediateH1 := opts.height * 4
		intermediateW2 := opts.width * 2
		intermediateH2 := opts.height * 2

		// Step 1: Initial downscale with CatmullRom for smoothness
		intermediate1 := imaging.Fit(sourceImg, intermediateW1, intermediateH1, imaging.CatmullRom)
		
		// Step 2: Second downscale with Box filter
		intermediate2 := imaging.Fit(intermediate1, intermediateW2, intermediateH2, imaging.Box)
		
		// Step 3: Final scale with Lanczos for maximum sharpness
		thumb = imaging.Fit(intermediate2, opts.width, opts.height, imaging.Lanczos)
	} else if maxScaleFactor > 2.0 {
		// For significant downscaling (>2x), use two-step approach
		intermediateW := opts.width * 2
		intermediateH := opts.height * 2

		// Step 1: Scale to intermediate size with CatmullRom for smooth downsampling
		intermediate := imaging.Fit(sourceImg, intermediateW, intermediateH, imaging.CatmullRom)

		// Step 2: Final scale with Lanczos for sharpness
		thumb = imaging.Fit(intermediate, opts.width, opts.height, imaging.Lanczos)
	} else {
		// For smaller scale factors, use Lanczos directly for best sharpness
		thumb = imaging.Fit(sourceImg, opts.width, opts.height, imaging.Lanczos)
	}

	// Enhanced sharpening for better clarity
	// Apply adaptive sharpening based on thumbnail size
	if opts.width >= 300 || opts.height >= 300 {
		// Larger thumbnails: moderate sharpening
		thumb = imaging.Sharpen(thumb, 0.7)
	} else if opts.width >= 150 || opts.height >= 150 {
		// Medium thumbnails: stronger sharpening for clarity
		thumb = imaging.Sharpen(thumb, 1.0)
	} else {
		// Small thumbnails: subtle sharpening to avoid artifacts
		thumb = imaging.Sharpen(thumb, 0.3)
	}
	
	// Apply slight contrast adjustment for better visual pop
	thumb = imaging.AdjustContrast(thumb, 5) // Subtle contrast boost

	// Convert to RGBA for WebP encoding
	thumbRGBA := image.NewRGBA(thumb.Bounds())
	draw.Draw(thumbRGBA, thumbRGBA.Bounds(), thumb, image.Point{}, draw.Src)

	// No cursor drawing - we're zooming in on the cursor area instead

	// Use lossless WebP encoding for maximum quality
	// This ensures no quality loss from compression
	var buf []byte
	buf, err = webp.EncodeLosslessRGBA(thumbRGBA) // Lossless encoding for perfect quality
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// ----------------- helpers -----------------

// findContentBounds finds the actual content area in an image by detecting non-transparent pixels
func findContentBounds(img *image.RGBA) image.Rectangle {
	bounds := img.Bounds()
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y

	hasContent := false
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			offset := (y-bounds.Min.Y)*img.Stride + (x-bounds.Min.X)*4
			// Check if pixel is not fully transparent (alpha > 0) or not black
			alpha := img.Pix[offset+3]
			r := img.Pix[offset]
			g := img.Pix[offset+1]
			b := img.Pix[offset+2]

			// Consider a pixel as content if it's not transparent or not pure black
			if alpha > 0 || r > 0 || g > 0 || b > 0 {
				hasContent = true
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}

	if !hasContent {
		return image.Rectangle{} // Empty rectangle if no content found
	}

	// Add 1 to max values because image.Rectangle is exclusive on the Max side
	return image.Rect(minX, minY, maxX+1, maxY+1)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// drawCursor draws a simple cursor indicator at the specified position
func drawCursor(img *image.RGBA, x, y int) {
	bounds := img.Bounds()

	// Cursor colors
	white := [4]uint8{255, 255, 255, 255}
	black := [4]uint8{0, 0, 0, 255}

	// Draw a crosshair cursor with outline for visibility
	cursorSize := 12

	// Horizontal line (with black outline)
	for i := -cursorSize; i <= cursorSize; i++ {
		px := x + i
		if px >= bounds.Min.X && px < bounds.Max.X {
			// Black outline
			if y-1 >= bounds.Min.Y && y-1 < bounds.Max.Y {
				setPixel(img, px, y-1, black)
			}
			if y+1 >= bounds.Min.Y && y+1 < bounds.Max.Y {
				setPixel(img, px, y+1, black)
			}
			// White center
			if y >= bounds.Min.Y && y < bounds.Max.Y && i != 0 {
				setPixel(img, px, y, white)
			}
		}
	}

	// Vertical line (with black outline)
	for i := -cursorSize; i <= cursorSize; i++ {
		py := y + i
		if py >= bounds.Min.Y && py < bounds.Max.Y {
			// Black outline
			if x-1 >= bounds.Min.X && x-1 < bounds.Max.X {
				setPixel(img, x-1, py, black)
			}
			if x+1 >= bounds.Min.X && x+1 < bounds.Max.X {
				setPixel(img, x+1, py, black)
			}
			// White center
			if x >= bounds.Min.X && x < bounds.Max.X && i != 0 {
				setPixel(img, x, py, white)
			}
		}
	}

	// Draw center dot
	if x >= bounds.Min.X && x < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y {
		setPixel(img, x, y, black)
	}
}

// setPixel sets a pixel in an RGBA image
func setPixel(img *image.RGBA, x, y int, color [4]uint8) {
	if x < img.Bounds().Min.X || x >= img.Bounds().Max.X ||
		y < img.Bounds().Min.Y || y >= img.Bounds().Max.Y {
		return
	}
	offset := (y-img.Bounds().Min.Y)*img.Stride + (x-img.Bounds().Min.X)*4
	copy(img.Pix[offset:offset+4], color[:])
}

// drawCursorBitmap draws the actual cursor bitmap at the specified position
func drawCursorBitmap(dst *image.RGBA, x, y int, cursor *image.RGBA, hotspotX, hotspotY int) {
	if cursor == nil {
		return
	}

	// Adjust position for hotspot (the point that corresponds to the actual cursor position)
	drawX := x - hotspotX
	drawY := y - hotspotY

	// Calculate the source and destination rectangles
	srcBounds := cursor.Bounds()
	dstBounds := dst.Bounds()

	// Clip to destination bounds
	drawRect := image.Rect(drawX, drawY, drawX+srcBounds.Dx(), drawY+srcBounds.Dy())
	clippedRect := drawRect.Intersect(dstBounds)

	if clippedRect.Empty() {
		return
	}

	// Draw cursor with alpha blending
	for py := clippedRect.Min.Y; py < clippedRect.Max.Y; py++ {
		for px := clippedRect.Min.X; px < clippedRect.Max.X; px++ {
			// Calculate source pixel position
			srcX := px - drawX
			srcY := py - drawY

			if srcX < 0 || srcX >= srcBounds.Dx() || srcY < 0 || srcY >= srcBounds.Dy() {
				continue
			}

			// Get source pixel
			srcOffset := srcY*cursor.Stride + srcX*4
			if srcOffset+3 >= len(cursor.Pix) {
				continue
			}

			srcR := cursor.Pix[srcOffset]
			srcG := cursor.Pix[srcOffset+1]
			srcB := cursor.Pix[srcOffset+2]
			srcA := cursor.Pix[srcOffset+3]

			if srcA == 0 {
				continue // Skip transparent pixels
			}

			// Get destination pixel
			dstOffset := (py-dstBounds.Min.Y)*dst.Stride + (px-dstBounds.Min.X)*4
			if dstOffset+3 >= len(dst.Pix) {
				continue
			}

			if srcA == 255 {
				// Opaque pixel - direct copy
				dst.Pix[dstOffset] = srcR
				dst.Pix[dstOffset+1] = srcG
				dst.Pix[dstOffset+2] = srcB
				dst.Pix[dstOffset+3] = 255
			} else {
				// Alpha blending
				dstR := dst.Pix[dstOffset]
				dstG := dst.Pix[dstOffset+1]
				dstB := dst.Pix[dstOffset+2]
				dstA := dst.Pix[dstOffset+3]

				alpha := uint32(srcA)
				invAlpha := 255 - alpha

				dst.Pix[dstOffset] = uint8((uint32(srcR)*alpha + uint32(dstR)*invAlpha) / 255)
				dst.Pix[dstOffset+1] = uint8((uint32(srcG)*alpha + uint32(dstG)*invAlpha) / 255)
				dst.Pix[dstOffset+2] = uint8((uint32(srcB)*alpha + uint32(dstB)*invAlpha) / 255)
				dst.Pix[dstOffset+3] = uint8(min(255, uint32(dstA)+alpha))
			}
		}
	}
}
