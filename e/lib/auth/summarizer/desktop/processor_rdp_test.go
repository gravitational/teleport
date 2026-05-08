//go:build desktop_access_rdp || rust_rdp_decoder

package desktop

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv/desktop/rdpstate/rdpstatetest"
)

func TestDesktopRecordingProcessor_Dedup_RefreshesPendingSnapshotOnSmallEdits(t *testing.T) {
	t.Parallel()

	d := NewRecordingProcessor(NewGlyphCache())
	defer d.Release()

	ts := time.Unix(1_700_000_000, 0).UTC()

	_, err := d.ProcessEvent(&apievents.WindowsDesktopSessionStart{
		Metadata: apievents.Metadata{Time: ts},
	})
	require.NoError(t, err)

	// 800x600 sample grid: stepX=12, stepY=9. A 5x5 update at (1,1) sits entirely between sample
	// rows/columns, so the digest is unchanged. The dirty fraction (25/480000 ≈ 0.005%) is below
	// dirtyRectMinFraction. Both halves of the dedup predicate falsely match.
	hello, err := rdpstatetest.EncodeTDPBServerHello(800, 600)
	require.NoError(t, err)
	hello.Metadata = apievents.Metadata{Time: ts.Add(100 * time.Millisecond)}
	_, err = d.ProcessEvent(hello)
	require.NoError(t, err)
	require.NotNil(t, d.pending, "first capture should set pending")

	// Pixel value as captured for the initial blank framebuffer.
	blankPixel := d.pending.img.RGBAAt(2, 2)

	// Off-grid 5x5 red bitmap: triggers settled (1.6s gap, pduBytesSinceLast > 0) past the
	// minInterval floor. captureFrame will hit the dedup branch.
	bitmap, err := rdpstatetest.EncodeTDPBFastPathPDU(
		rdpstatetest.BuildBitmapPDU(1, 1, 5, 5, rdpstatetest.RGB565Red),
	)
	require.NoError(t, err)
	bitmap.Metadata = apievents.Metadata{Time: ts.Add(1700 * time.Millisecond)}
	res, err := d.ProcessEvent(bitmap)
	require.NoError(t, err)
	require.Nil(t, res, "dedup should suppress emission")
	require.Equal(t, 1700*time.Millisecond, d.pending.endTime, "dedup should extend pending.endTime")

	require.NotEqual(t, blankPixel, d.pending.img.RGBAAt(2, 2),
		"dedup must refresh pending.img so off-sample-grid edits aren't dropped at flush")
}

func TestDesktopRecordingProcessor_Flush_CapturesFinalStateAfterMinInterval(t *testing.T) {
	t.Parallel()

	d := NewRecordingProcessor(NewGlyphCache())
	defer d.Release()

	ts := time.Unix(1_700_000_000, 0).UTC()

	_, err := d.ProcessEvent(&apievents.WindowsDesktopSessionStart{
		Metadata: apievents.Metadata{Time: ts},
	})
	require.NoError(t, err)

	hello, err := rdpstatetest.EncodeTDPBServerHello(800, 600)
	require.NoError(t, err)
	hello.Metadata = apievents.Metadata{Time: ts.Add(100 * time.Millisecond)}
	res, err := d.ProcessEvent(hello)
	require.NoError(t, err)
	require.Nil(t, res, "first capture stores pending, does not emit")
	require.NotNil(t, d.pending, "first capture should set pending")
	blankHash := d.pending.hash

	bitmap, err := rdpstatetest.EncodeTDPBFastPathPDU(
		rdpstatetest.BuildBitmapPDU(0, 0, 100, 100, rdpstatetest.RGB565Red),
	)
	require.NoError(t, err)
	bitmap.Metadata = apievents.Metadata{Time: ts.Add(500 * time.Millisecond)}
	res, err = d.ProcessEvent(bitmap)
	require.NoError(t, err)
	require.Nil(t, res, "minInterval should block re-capture")
	require.Equal(t, blankHash, d.pending.hash, "pending still reflects the blank frame")

	results, err := d.Flush()
	require.NoError(t, err)
	require.Len(t, results, 2, "Flush should emit pending plus a fresh capture of the final state")
	require.LessOrEqual(t, results[0].StartTime, results[1].StartTime, "results should be in chronological order")
}
