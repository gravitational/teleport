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
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
)

// rdpThumbnailCapture handles thumbnail generation from RDP Fast Path PDU events
type rdpThumbnailCapture struct {
	// circular buffer of snapshots
	snapshots        []frameSnapshot
	frameIndex       uint32
	currentFrame     *image.RGBA
	mu               sync.RWMutex
	interval         time.Duration
	keyframes        map[uint32]*frameData
	lastSnapshotTime time.Duration
	frameChanged     bool

	currentFrameRef  uint32
	lastKeyframe     uint32
	lastKeyframeTime time.Duration

	cursorX atomic.Int32
	cursorY atomic.Int32

	// Accumulated screen updates from RDP PDUs
	pendingUpdates []screenUpdate

	// Track screen dimensions
	screenWidth  uint16
	screenHeight uint16
}

type screenUpdate struct {
	rect image.Rectangle
	data []byte // raw bitmap data from RDP
}

func newRDPThumbnailCapture(interval time.Duration, size int) *rdpThumbnailCapture {
	return &rdpThumbnailCapture{
		snapshots:      make([]frameSnapshot, 0, size),
		keyframes:      make(map[uint32]*frameData),
		interval:       interval,
		pendingUpdates: make([]screenUpdate, 0),
	}
}

func (d *rdpThumbnailCapture) handleEvent(event *apievents.DesktopRecording) error {
	msg, err := tdp.Decode(event.Message)
	if err != nil {
		return trace.Wrap(err, "failed to decode desktop recording event")
	}

	switch m := msg.(type) {
	case tdp.RDPFastPathPDU:
		// RDP Fast Path PDUs contain screen update data in RDP protocol format
		// We need to decode these to extract bitmap updates
		if err := d.processRDPFastPathPDU(m); err != nil {
			return trace.Wrap(err, "failed to process RDP Fast Path PDU")
		}
		d.frameChanged = true

	case tdp.PNG2Frame:
		// Some deployments might still use PNG2Frame
		if err := d.processPNG2Frame(m); err != nil {
			return trace.Wrap(err, "failed to process PNG2 frame")
		}
		d.frameChanged = true

	case tdp.PNGFrame:
		// Legacy PNG frame support
		d.applyPNGFrame(&m)
		d.frameChanged = true

	case tdp.MouseMove:
		d.updateCursorPosition(int(m.X), int(m.Y))

	case tdp.ConnectionActivated:
		// Track screen dimensions from connection activation
		d.screenWidth = m.ScreenWidth
		d.screenHeight = m.ScreenHeight
		d.initializeCanvas(int(m.ScreenWidth), int(m.ScreenHeight))
	}

	ms := time.Duration(event.DelayMilliseconds) * time.Millisecond

	if ms-d.lastSnapshotTime >= d.interval {
		d.lastSnapshotTime = ms

		if d.frameChanged {
			// Apply pending updates to current frame
			d.applyPendingUpdates()

			// Store a keyframe periodically or when significant changes occur
			if d.shouldStoreKeyframe(ms) {
				keyframeRef, err := d.storeKeyframe(ms)
				if err != nil {
					return trace.Wrap(err, "failed to store keyframe")
				}
				d.currentFrameRef = keyframeRef
			}

			d.frameChanged = false
		}

		d.captureSnapshot(ms, d.currentFrameRef)
	}

	return nil
}

func (d *rdpThumbnailCapture) processRDPFastPathPDU(pdu tdp.RDPFastPathPDU) error {
	// RDP Fast Path PDUs contain encoded screen updates
	// The structure is complex and requires proper RDP protocol parsing
	// For now, we'll store the raw PDU and note that proper decoding
	// would require integrating with an RDP decoder library

	// This is a simplified placeholder - actual implementation would need
	// to parse the RDP Fast Path Update PDU structure to extract:
	// - Update type (bitmap, orders, etc.)
	// - Rectangle coordinates
	// - Bitmap data

	// For the MVP, we can just mark that we received an update
	d.mu.Lock()
	defer d.mu.Unlock()

	// Store that we received an update (simplified)
	d.pendingUpdates = append(d.pendingUpdates, screenUpdate{
		rect: image.Rect(0, 0, int(d.screenWidth), int(d.screenHeight)),
		data: pdu,
	})

	return nil
}

func (d *rdpThumbnailCapture) processPNG2Frame(frame tdp.PNG2Frame) error {
	// PNG2Frame contains actual PNG data with coordinates
	left := frame.Left()
	top := frame.Top()
	right := frame.Right()
	bottom := frame.Bottom()
	pngData := frame.Data()

	// Decode PNG data
	img, _, err := image.Decode(bytes.NewReader(pngData))
	if err != nil {
		return trace.Wrap(err, "failed to decode PNG data")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Initialize canvas if needed
	if d.currentFrame == nil {
		d.initializeCanvasLocked(int(right), int(bottom))
	}

	// Draw the decoded image to the current frame
	bounds := image.Rect(int(left), int(top), int(right), int(bottom))
	draw.Draw(d.currentFrame, bounds, img, image.Point{}, draw.Src)

	return nil
}

func (d *rdpThumbnailCapture) applyPNGFrame(frame *tdp.PNGFrame) {
	if frame.Img == nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	bounds := frame.Img.Bounds()

	if d.currentFrame == nil {
		canvas := image.Rect(0, 0, bounds.Max.X, bounds.Max.Y)
		if bounds.Min.X == 0 && bounds.Min.Y == 0 {
			canvas = image.Rect(0, 0, bounds.Dx(), bounds.Dy())
		}
		d.currentFrame = image.NewRGBA(canvas)
	}

	// Expand canvas if needed
	c := d.currentFrame.Bounds()
	if bounds.Max.X > c.Max.X || bounds.Max.Y > c.Max.Y {
		newW := max(c.Max.X, bounds.Max.X)
		newH := max(c.Max.Y, bounds.Max.Y)
		newFrame := image.NewRGBA(image.Rect(0, 0, newW, newH))
		draw.Draw(newFrame, c, d.currentFrame, image.Point{}, draw.Src)
		d.currentFrame = newFrame
	}

	// Draw the frame
	draw.Draw(d.currentFrame, bounds, frame.Img, bounds.Min, draw.Src)
}

func (d *rdpThumbnailCapture) initializeCanvas(width, height int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.initializeCanvasLocked(width, height)
}

func (d *rdpThumbnailCapture) initializeCanvasLocked(width, height int) {
	if d.currentFrame == nil {
		d.currentFrame = image.NewRGBA(image.Rect(0, 0, width, height))
	}
}

func (d *rdpThumbnailCapture) applyPendingUpdates() {
	d.mu.Lock()
	defer d.mu.Unlock()

	// In a full implementation, this would decode RDP bitmap updates
	// and apply them to the current frame
	// For now, we just clear the pending updates
	d.pendingUpdates = d.pendingUpdates[:0]
}

func (d *rdpThumbnailCapture) shouldStoreKeyframe(now time.Duration) bool {
	// Store first frame as keyframe
	if d.lastKeyframe == 0 {
		return true
	}

	// Store keyframe every 3 seconds
	secondsSinceKF := (now - d.lastKeyframeTime).Seconds()
	return secondsSinceKF >= 3.0
}

func (d *rdpThumbnailCapture) storeKeyframe(now time.Duration) (uint32, error) {
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

	return d.frameIndex, nil
}

func (d *rdpThumbnailCapture) updateCursorPosition(x, y int) {
	d.cursorX.Store(int32(x))
	d.cursorY.Store(int32(y))
}

func (d *rdpThumbnailCapture) captureSnapshot(ts time.Duration, frameRef uint32) {
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

func (d *rdpThumbnailCapture) GenerateThumbnail(index int, opts thumbnailOptions) ([]byte, error) {
	d.mu.RLock()
	if index >= len(d.snapshots) {
		d.mu.RUnlock()
		return nil, nil
	}
	snap := d.snapshots[index]
	frameRef := snap.frameRef
	d.mu.RUnlock()

	if frameRef == 0 {
		return nil, nil
	}

	// Get the keyframe
	d.mu.RLock()
	kf, ok := d.keyframes[frameRef]
	d.mu.RUnlock()

	if !ok || kf.image == nil {
		return nil, fmt.Errorf("frame %d not found", frameRef)
	}

	rgba := kf.image
	b := rgba.Bounds()

	// Handle zoom factor
	if opts.zoomFactor <= 0 {
		opts.zoomFactor = 1.0
	}
	zoomedW := int(float64(b.Dx()) / opts.zoomFactor)
	zoomedH := int(float64(b.Dy()) / opts.zoomFactor)
	if zoomedW < 1 {
		zoomedW = 1
	}
	if zoomedH < 1 {
		zoomedH = 1
	}

	// Center on cursor position
	cx := clamp(snap.cursorX-zoomedW/2, 0, b.Dx()-zoomedW)
	cy := clamp(snap.cursorY-zoomedH/2, 0, b.Dy()-zoomedH)
	crop := image.Rect(cx, cy, cx+zoomedW, cy+zoomedH)

	// Crop and resize
	cropped := imaging.Crop(rgba, crop)
	thumb := imaging.Resize(cropped, opts.width, opts.height, imaging.Linear)

	// Encode to WebP
	buf, err := webp.EncodeRGBA(thumb, 70)
	if err != nil {
		return nil, trace.Wrap(err, "failed to encode WebP")
	}
	return buf, nil
}

// RDPFastPathPDU decoder helper
// This would need to integrate with an RDP decoder library like IronRDP
// to properly extract bitmap updates from the Fast Path PDUs
type rdpDecoder struct {
	// This would contain the actual RDP decoder state
	// For now, it's a placeholder for the integration point
}

func (d *rdpDecoder) decodeFastPathPDU(pdu []byte) ([]screenUpdate, error) {
	// Placeholder for RDP Fast Path PDU decoding
	// A real implementation would:
	// 1. Parse the Fast Path header
	// 2. Extract update PDUs
	// 3. Decode bitmap/order updates
	// 4. Return screen update rectangles with bitmap data

	// Fast Path PDU structure (simplified):
	// - Header (1-2 bytes)
	// - Length (1-2 bytes)
	// - Update PDUs
	//   - Update type
	//   - Update data (bitmaps, orders, etc.)

	if len(pdu) < 4 {
		return nil, fmt.Errorf("PDU too short")
	}

	// This is a stub - actual implementation would parse the RDP protocol
	return nil, nil
}

// Helper to decode basic Fast Path PDU header
func decodeFastPathHeader(data []byte) (action byte, flags byte, length int, err error) {
	if len(data) < 2 {
		return 0, 0, 0, fmt.Errorf("insufficient data for fast path header")
	}

	header := data[0]
	action = (header >> 2) & 0x03
	flags = header & 0x03

	// Parse length (can be 1 or 2 bytes)
	length = int(data[1])
	offset := 2
	if length&0x80 != 0 {
		if len(data) < 3 {
			return 0, 0, 0, fmt.Errorf("insufficient data for 2-byte length")
		}
		length = ((int(data[1]) & 0x7F) << 8) | int(data[2])
		offset = 3
	}

	// Validate we have enough data
	if len(data) < offset+length {
		return 0, 0, 0, fmt.Errorf("PDU length mismatch")
	}

	return action, flags, length, nil
}

// Constants for RDP Fast Path update types
const (
	fastpathUpdateOrders      = 0x00
	fastpathUpdateBitmap      = 0x01
	fastpathUpdatePalette     = 0x02
	fastpathUpdateSynchronize = 0x03
	fastpathUpdateSurfcmds    = 0x04
	fastpathUpdatePtrNull     = 0x05
	fastpathUpdatePtrDefault  = 0x06
	fastpathUpdatePtrPosition = 0x08
	fastpathUpdateColor       = 0x09
	fastpathUpdateCached      = 0x0A
	fastpathUpdatePointer     = 0x0B
)

// Parse a Fast Path Update PDU to extract update type and data
func parseFastPathUpdate(data []byte) (updateType byte, updateData []byte, err error) {
	if len(data) < 3 {
		return 0, nil, fmt.Errorf("update PDU too short")
	}

	// Update header
	updateHeader := data[0]
	updateType = updateHeader & 0x0F
	fragmentation := (updateHeader >> 4) & 0x03
	compression := (updateHeader >> 6) & 0x03

	// Skip size field (2 bytes)
	if len(data) < 3 {
		return 0, nil, fmt.Errorf("missing update size")
	}

	size := binary.LittleEndian.Uint16(data[1:3])

	if len(data) < int(3+size) {
		return 0, nil, fmt.Errorf("update data truncated")
	}

	updateData = data[3 : 3+size]

	// Handle compression if needed
	if compression != 0 {
		// Would need to decompress using RDP compression algorithm
		return updateType, updateData, fmt.Errorf("compressed updates not yet supported")
	}

	// Handle fragmentation if needed
	if fragmentation != 0 {
		// Would need to reassemble fragments
		return updateType, updateData, fmt.Errorf("fragmented updates not yet supported")
	}

	return updateType, updateData, nil
}
