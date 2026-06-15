package desktop

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"image"
	"image/png"
	"math"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/image/draw"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv/desktop/rdpstate"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
)

const (
	// Anthropic Claude vision limits; we resize ahead of time so the server doesn't downscale and waste bytes.
	screenshotMaxDimension = 1568
	screenshotMaxPixels    = 1_150_000

	// baseScreenshotInterval is the nominal capture interval, adapted by session duration.
	baseScreenshotInterval = 3 * time.Second
	// screenshotMinInterval is the absolute minimum time between any two screenshots.
	screenshotMinInterval = 1500 * time.Millisecond

	// Session age tiers used by adaptiveInterval; capture frequency relaxes as a session ages.
	earlySessionScreenshots = 20
	earlySessionAge         = 2 * time.Minute
	midSessionAge           = 5 * time.Minute
	longSessionAge          = 10 * time.Minute

	// settledQuietPeriod is the silence threshold after a PDU burst that fires a "settled" capture.
	settledQuietPeriod = 500 * time.Millisecond

	// minActiveWindowFraction: active-window-to-screen denominator below which the cluster is treated as noise.
	minActiveWindowFraction = 10
	// dirtyRectMinFraction is the minimum dirty-rect fraction used for cropping; low (0.5%) to capture typing.
	dirtyRectMinFraction = 0.005

	// glyphPadding is extra horizontal padding when expanding a crop to fit the timestamp text.
	glyphPadding = 8
	// dirtyRectPadding pads the dirty bounding box during cropping so the LLM has surrounding context.
	dirtyRectPadding = 100

	// hashSampleCount is the number of pixels sampled per dimension for the dedup hash.
	hashSampleCount = 64

	// bytesPerPixel is the size of a single RGBA pixel in image.RGBA's Pix slice.
	bytesPerPixel = 4

	// maxTimestampLabelChars bounds the crop-snapshot width to fit "Screenshot taken at HHHHH:MM:SS - HHHHH:MM:SS".
	maxTimestampLabelChars = 50

	// screenshotLabelPrefix is the leading text of every timestamp label rendered onto a screenshot.
	screenshotLabelPrefix = "Screenshot taken at "
)

// RecordingProcessor consumes events from a desktop session recording and emits PNG screenshots suitable for
// sending to an LLM.
//
// Capture triggers (highest priority first):
//  1. First capture: fires immediately on the first DesktopRecording event and forces a full-screen shot, so the session
//     always has a baseline frame. This can be a blank screen (if the recording starts before the desktop is visible)
//     or a non-blank screen (if the recording starts after the desktop is visible), but it ensures the processor's
//     internal state is initialized, and we have a reference frame for future change detection.
//  2. Settled after activity: fires when a burst of PDUs is followed by at least settledQuietPeriod of silence.
//     Targets the moment a command finishes, a dialog finishes rendering, or typing pauses.
//  3. Adaptive time interval: safety net for sessions with sustained activity that never settle. The interval scales
//     with session age (short session = frequent captures, long session = sparse captures) and shortens when an
//     active window has been detected. See adaptiveInterval for the buckets.
//
// A hard floor of screenshotMinInterval is applied before any trigger fires, so no two captures can be closer
// than that regardless of triggers.
//
// Each capture runs the accumulated RDP update regions plus the current cursor through windowTracker to detect the
// active window. chooseCropBounds then decides whether to keep the full screen (no/small active window, forced
// full-screen, or padding that swallows the whole screen) or crop to a padded active-window rectangle.
//
// captureFrame does not emit the current frame directly. It stores the frame as d.pending and emits the previous
// pending, if any. The observation window between capture and emission is used for two things:
//  1. Dedup: if a later capture has the same sampled pixel hash and a negligible dirty fraction, the pending's
//     end time is extended and no new screenshot is produced. Runs of identical frames collapse into a single
//     screenshot with a time range label (e.g. "Screenshot taken at 0:05 - 0:15").
//  2. Time-range labels: the stamped label uses the accumulated [start, end] range once the frame finally
//     differs. Flush emits the last pending at end of stream so the final screen state is never dropped.
//
// emitPending composites a timestamp bar above the cropped frame and scales the result via fitImageDimensions so it
// fits within LLM vision constraints (max dimension and total pixel budget).
type RecordingProcessor struct {
	state         *rdpstate.RDPState
	windowTracker *windowTracker
	glyphs        *GlyphCache

	startTime          time.Time
	lastScreenshotTime time.Time
	hasFrame           bool

	pduBytesSinceLast        int
	screenshotsTaken         int
	lastDesktopRecordingTime time.Time
	quietGap                 time.Duration

	pending         *pendingScreenshot
	hadActiveWindow bool

	encoder      *png.Encoder
	finalScratch []uint8      // reusable Pix backing for the composed emit image
	encodeBuf    bytes.Buffer // reusable PNG output buffer
}

// pendingScreenshot holds a captured frame held until the next capture, then composited with a duration label and emitted.
type pendingScreenshot struct {
	img        *image.RGBA
	cropBounds image.Rectangle
	hash       uint64
	startTime  time.Duration // relative to session start
	endTime    time.Duration // updated when the same hash is seen again
	fullScreen bool
}

// ScreenshotResult holds a screenshot produced by ProcessEvent or Flush.
type ScreenshotResult struct {
	// PNG is the PNG-encoded screenshot with timestamp bar composited.
	PNG []byte
	// Width and Height are the pixel dimensions of the encoded image.
	Width, Height int
	// StartTime is the relative timestamp (from session start) when this screen state was first captured.
	StartTime time.Duration
	// EndTime is the relative timestamp when this screen state was last seen (i.e. the screenshot that replaced it).
	// Equal to StartTime if the state was only captured once.
	EndTime time.Duration
}

// NewRecordingProcessor processes a desktop recording and emits screenshots cropped to the detected active window.
func NewRecordingProcessor(glyphs *GlyphCache) *RecordingProcessor {
	return &RecordingProcessor{
		state:         rdpstate.New(),
		glyphs:        glyphs,
		windowTracker: newWindowTracker(),
		encoder:       tdp.PNGEncoder(),
	}
}

// StartTime returns the session start time recorded from the session start event.
func (d *RecordingProcessor) StartTime() time.Time {
	return d.startTime
}

// Release frees the underlying decoder.
func (d *RecordingProcessor) Release() {
	d.state.Release()
}

// ProcessEvent handles a single audit event and returns a screenshot if one was emitted, or nil if not.
func (d *RecordingProcessor) ProcessEvent(evt apievents.AuditEvent) (*ScreenshotResult, error) {
	switch evt := evt.(type) {
	case *apievents.WindowsDesktopSessionStart:
		d.handleSessionStart(evt.GetTime())

	case *apievents.DesktopRecording:
		return d.handleDesktopRecording(evt)
	}

	return nil, nil
}

// Flush emits any remaining screenshots at end of stream, including a fresh capture of the decoder's final state
// when it differs from the pending. Returns up to two results in chronological order.
func (d *RecordingProcessor) Flush() ([]ScreenshotResult, error) {
	var results []ScreenshotResult

	if d.hasFrame {
		emitted, err := d.captureFrame(d.lastDesktopRecordingTime, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if emitted != nil {
			results = append(results, *emitted)
		}
	}

	if d.pending != nil {
		r, err := d.emitPending()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		results = append(results, r)
	}

	return results, nil
}

func (d *RecordingProcessor) handleSessionStart(startTime time.Time) {
	d.startTime = startTime
	d.lastDesktopRecordingTime = startTime
}

func (d *RecordingProcessor) handleDesktopRecording(evt *apievents.DesktopRecording) (*ScreenshotResult, error) {
	eventTime := evt.GetTime()

	d.quietGap = eventTime.Sub(d.lastDesktopRecordingTime)
	d.lastDesktopRecordingTime = eventTime
	d.pduBytesSinceLast += len(evt.Message) + len(evt.TDPBMessage)

	if err := d.state.HandleMessage(evt); err != nil {
		return nil, trace.Wrap(err, "processing desktop recording event")
	}

	if !d.hasFrame && d.state.Image() != nil {
		d.hasFrame = true
	}

	if !d.hasFrame {
		return nil, nil
	}

	if !d.lastScreenshotTime.IsZero() && eventTime.Sub(d.lastScreenshotTime) < screenshotMinInterval {
		return nil, nil
	}

	capture, forceFullScreen := d.evaluateTrigger(eventTime)
	if !capture {
		return nil, nil
	}

	return d.captureFrame(eventTime, forceFullScreen)
}

// evaluateTrigger picks the highest-priority capture trigger that fires now (see RecordingProcessor for the order).
func (d *RecordingProcessor) evaluateTrigger(eventTime time.Time) (capture, forceFullScreen bool) {
	switch {
	case d.lastScreenshotTime.IsZero():
		return true, true
	case d.shouldTriggerSettled():
		d.quietGap = 0
		return true, false
	case d.shouldTriggerTimeInterval(eventTime):
		return true, false
	}
	return false, false
}

// captureFrame stores the current decoder frame as pending and emits the prior pending (if any).
func (d *RecordingProcessor) captureFrame(eventTime time.Time, forceFullScreen bool) (*ScreenshotResult, error) {
	img := d.state.Image()
	if img == nil {
		return nil, nil
	}

	bounds := img.Bounds()
	screenArea := bounds.Dx() * bounds.Dy()

	activeWindow, unionDirty := d.analyzeFrame()
	d.hadActiveWindow = isActiveWindowSignificant(activeWindow, screenArea)

	screenDirtyFrac := rectFraction(unionDirty, screenArea)
	cropBounds := chooseCropBounds(bounds, activeWindow, forceFullScreen)

	currentHash := sampleImageHash(img)
	relativeTime := eventTime.Sub(d.startTime)

	// Dedup: hash unchanged and dirty fraction negligible - extend the pending's end time and suppress emission.
	if d.pending != nil && currentHash == d.pending.hash && screenDirtyFrac < dirtyRectMinFraction {
		d.pending.endTime = relativeTime

		// Fixed-grid hash can miss off-grid edits (single chars, caret blinks); refresh the snapshot when there's any dirt.
		if !unionDirty.Empty() {
			b := d.pending.img.Bounds()
			//nolint:staticcheck // err is always non-nil in nop build but nil in RDP build
			fresh, err := d.state.ResizeCrop(
				uint16(b.Min.X), uint16(b.Min.Y),
				uint16(b.Dx()), uint16(b.Dy()),
				uint16(b.Dx()), uint16(b.Dy()),
				true,
			)
			if err == nil { //nolint:staticcheck // err is always non-nil in nop build but nil in RDP build
				copy(d.pending.img.Pix, fresh.Pix)
			}
		}

		d.lastScreenshotTime = eventTime
		d.pduBytesSinceLast = 0

		return nil, nil
	}

	// Emit the previous pending first so its snapshot Pix buffer becomes available for reuse below.
	var emitted *ScreenshotResult
	if d.pending != nil {
		r, err := d.emitPending()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		emitted = &r
	}

	// Pre-expand the crop to the worst-case label width so emit never reads pixels outside the snapshot.
	worstCaseMinWidth := maxTimestampLabelChars*d.glyphs.glyphW + glyphPadding
	snapshotBounds := expandCropForTimestamp(cropBounds, worstCaseMinWidth, bounds)

	//nolint:staticcheck // err is always non-nil in nop build but nil in RDP build
	snapshot, err := d.state.ResizeCrop(
		uint16(snapshotBounds.Min.X), uint16(snapshotBounds.Min.Y),
		uint16(snapshotBounds.Dx()), uint16(snapshotBounds.Dy()),
		uint16(snapshotBounds.Dx()), uint16(snapshotBounds.Dy()),
		true,
	)
	if err != nil { //nolint:staticcheck // err is always non-nil in nop build but nil in RDP build
		return nil, trace.Wrap(err)
	}
	// ResizeCrop returns an image with origin (0,0); reinterpret it under the original screen
	// coord system so emit's cropBounds-based indexing keeps working.
	snapshot.Rect = snapshotBounds

	d.pending = &pendingScreenshot{
		img:        snapshot,
		cropBounds: cropBounds,
		hash:       currentHash,
		startTime:  relativeTime,
		endTime:    relativeTime,
		fullScreen: forceFullScreen,
	}

	d.lastScreenshotTime = eventTime
	d.pduBytesSinceLast = 0
	d.screenshotsTaken++

	return emitted, nil
}

// analyzeFrame drains the decoder's accumulated update regions and current cursor through the window tracker, returning
// the detected active-window rectangle and the union of all update regions.
func (d *RecordingProcessor) analyzeFrame() (activeWindow, dirtyRegion image.Rectangle) {
	regions := d.state.UpdatedRegions()
	cursor := d.state.CursorState()

	d.windowTracker.Update(regions, image.Pt(int(cursor.X), int(cursor.Y)))
	d.state.ResetUpdatedRegions()

	activeWindow = d.windowTracker.ActiveWindow()
	dirtyRegion = d.windowTracker.UnionDirtyRect()

	d.windowTracker.Reset()

	return activeWindow, dirtyRegion
}

// emitPending composites the pending frame into a final PNG with a timestamp bar and returns it. Clears the pending state.
func (d *RecordingProcessor) emitPending() (ScreenshotResult, error) {
	p := d.pending
	d.pending = nil

	label := formatDurationLabel(p.startTime, p.endTime)
	minWidth := len(label)*d.glyphs.glyphW + glyphPadding
	// Clamp to the snapshot bounds: maxTimestampLabelChars bounds the label above, so any realistic label fits.
	cropBounds := expandCropForTimestamp(p.cropBounds, minWidth, p.img.Rect)

	cropW, cropH := cropBounds.Dx(), cropBounds.Dy()
	dstW, dstH := fitImageDimensions(cropW, cropH)
	finalH := dstH + timestampBarHeight
	final := d.acquireFinalImage(dstW, finalH)
	dstRect := image.Rect(0, timestampBarHeight, dstW, finalH)

	if dstW != cropW || dstH != cropH {
		draw.BiLinear.Scale(final, dstRect, p.img, cropBounds, draw.Src, nil)
	} else {
		draw.Draw(final, dstRect, p.img, cropBounds.Min, draw.Src)
	}

	d.glyphs.stampLabel(final, label)

	d.encodeBuf.Reset()
	if err := d.encoder.Encode(&d.encodeBuf, final); err != nil {
		return ScreenshotResult{}, trace.Wrap(err, "encoding screenshot to PNG")
	}

	// Clone the PNG bytes so subsequent emits can reuse encodeBuf without aliasing the returned slice.
	data := bytes.Clone(d.encodeBuf.Bytes())

	return ScreenshotResult{
		PNG:       data,
		Width:     dstW,
		Height:    finalH,
		StartTime: p.startTime,
		EndTime:   p.endTime,
	}, nil
}

// acquireFinalImage returns a zeroed w x h *image.RGBA backed by the processor's reusable final-image buffer.
func (d *RecordingProcessor) acquireFinalImage(w, h int) *image.RGBA {
	need := w * h * bytesPerPixel
	if cap(d.finalScratch) < need {
		d.finalScratch = make([]uint8, need)
	} else {
		d.finalScratch = d.finalScratch[:need]
		clear(d.finalScratch)
	}

	return &image.RGBA{
		Pix:    d.finalScratch,
		Stride: w * bytesPerPixel,
		Rect:   image.Rect(0, 0, w, h),
	}
}

// sampleImageHash returns an FNV-64a digest of ~hashSampleCount^2 pixels sampled from img.
func sampleImageHash(img *image.RGBA) uint64 {
	if img == nil {
		return 0
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w == 0 || h == 0 {
		return 0
	}

	stepX := w / hashSampleCount
	if stepX < 1 {
		stepX = 1
	}
	stepY := h / hashSampleCount
	if stepY < 1 {
		stepY = 1
	}

	rowStep := img.Stride * stepY
	colStep := bytesPerPixel * stepX
	colEnd := w * bytesPerPixel
	rowOff := img.PixOffset(bounds.Min.X, bounds.Min.Y)
	pixLen := len(img.Pix)
	hasher := fnv.New64a()
	for y := 0; y < h; y += stepY {
		for off := rowOff; off < rowOff+colEnd && off+bytesPerPixel <= pixLen; off += colStep {
			_, _ = hasher.Write(img.Pix[off : off+bytesPerPixel])
		}
		rowOff += rowStep
	}

	return hasher.Sum64()
}

// adaptiveInterval returns the minimum time between captures, tighter with an active window and looser as the session ages.
func (d *RecordingProcessor) adaptiveInterval(eventTime time.Time) time.Duration {
	elapsed := eventTime.Sub(d.startTime)

	if d.hadActiveWindow {
		switch {
		case d.screenshotsTaken < earlySessionScreenshots:
			return baseScreenshotInterval / 2
		case elapsed < earlySessionAge:
			return baseScreenshotInterval / 2
		case elapsed < midSessionAge:
			return baseScreenshotInterval
		case elapsed < longSessionAge:
			return 5 * time.Second
		default:
			return 7 * time.Second
		}
	}

	switch {
	case d.screenshotsTaken < earlySessionScreenshots:
		return baseScreenshotInterval
	case elapsed < earlySessionAge:
		return baseScreenshotInterval
	case elapsed < midSessionAge:
		return 7 * time.Second
	case elapsed < longSessionAge:
		return 10 * time.Second
	default:
		return 15 * time.Second
	}
}

// shouldTriggerSettled fires when a PDU burst is followed by at least settledQuietPeriod of silence.
func (d *RecordingProcessor) shouldTriggerSettled() bool {
	return d.quietGap >= settledQuietPeriod && d.pduBytesSinceLast > 0
}

// shouldTriggerTimeInterval fires when adaptiveInterval has elapsed since the last capture.
func (d *RecordingProcessor) shouldTriggerTimeInterval(eventTime time.Time) bool {
	if d.lastScreenshotTime.IsZero() {
		return true
	}

	return eventTime.Sub(d.lastScreenshotTime) >= d.adaptiveInterval(eventTime)
}

// chooseCropBounds returns the screenshot crop: full bounds for full-screen captures or insignificant active windows,
// otherwise the active window padded for surrounding context.
func chooseCropBounds(bounds, activeWindow image.Rectangle, forceFullScreen bool) image.Rectangle {
	if forceFullScreen {
		return bounds
	}

	screenArea := bounds.Dx() * bounds.Dy()
	if !isActiveWindowSignificant(activeWindow, screenArea) {
		return bounds
	}

	padded := padRect(activeWindow, dirtyRectPadding, bounds)
	if padded.Dx()*padded.Dy() >= screenArea {
		return bounds
	}

	return padded
}

// isActiveWindowSignificant reports whether activeWindow is large enough relative to screenArea to count as a real
// window rather than noise (tooltips, cursor-adjacent repaints). Below the threshold it's ignored by both crop and
// adaptive-interval decisions.
func isActiveWindowSignificant(activeWindow image.Rectangle, screenArea int) bool {
	if activeWindow.Empty() {
		return false
	}
	return activeWindow.Dx()*activeWindow.Dy() >= screenArea/minActiveWindowFraction
}

func rectFraction(r image.Rectangle, totalArea int) float64 {
	if r.Empty() || totalArea <= 0 {
		return 0
	}

	return float64(r.Dx()*r.Dy()) / float64(totalArea)
}

// padRect expands r by padding pixels on each side, clamped to maxBounds.
func padRect(r image.Rectangle, padding int, maxBounds image.Rectangle) image.Rectangle {
	r.Min.X -= padding
	r.Min.Y -= padding
	r.Max.X += padding
	r.Max.Y += padding

	return r.Intersect(maxBounds)
}

// formatDurationLabel returns a "Screenshot taken at start" label, or "start - end" when the two differ.
func formatDurationLabel(start, end time.Duration) string {
	startStr := FormatTimestamp(start)
	if start == end {
		return screenshotLabelPrefix + startStr
	}

	return screenshotLabelPrefix + startStr + " - " + FormatTimestamp(end)
}

// FormatTimestamp formats a duration as a human-readable timestamp.
func FormatTimestamp(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}

	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

// expandCropForTimestamp widens cropBounds to at least minWidth, centered on the original crop and clamped to screenBounds.
func expandCropForTimestamp(cropBounds image.Rectangle, minWidth int, screenBounds image.Rectangle) image.Rectangle {
	if cropBounds.Dx() >= minWidth {
		return cropBounds
	}

	deficit := minWidth - cropBounds.Dx()
	halfDeficit := deficit / 2

	cropBounds.Min.X -= halfDeficit
	cropBounds.Max.X += deficit - halfDeficit

	// Clamp to screen bounds.
	if cropBounds.Min.X < screenBounds.Min.X {
		shift := screenBounds.Min.X - cropBounds.Min.X
		cropBounds.Min.X += shift
		cropBounds.Max.X += shift
	}
	if cropBounds.Max.X > screenBounds.Max.X {
		shift := cropBounds.Max.X - screenBounds.Max.X
		cropBounds.Max.X -= shift
		cropBounds.Min.X -= shift
	}

	return cropBounds.Intersect(screenBounds)
}

// fitImageDimensions returns dimensions that, with the timestamp bar added, fit screenshotMaxDimension and
// screenshotMaxPixels while preserving aspect ratio.
func fitImageDimensions(w, h int) (int, int) {
	totalH := h + timestampBarHeight

	// Clamp long edge; floor scaled values to 1 so extreme aspect ratios can't truncate the other dimension to 0.
	if w > screenshotMaxDimension || totalH > screenshotMaxDimension {
		if w > totalH {
			totalH = max(totalH*screenshotMaxDimension/w, 1)
			w = screenshotMaxDimension
		} else {
			w = max(w*screenshotMaxDimension/totalH, 1)
			totalH = screenshotMaxDimension
		}
	}

	if pixels := w * totalH; pixels > screenshotMaxPixels {
		scale := math.Sqrt(float64(screenshotMaxPixels) / float64(pixels))
		w = max(int(float64(w)*scale), 1)
		totalH = max(int(float64(totalH)*scale), 1)
	}

	return w, max(totalH-timestampBarHeight, 1)
}
