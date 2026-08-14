//go:build desktop_access_rdp || rust_rdp_decoder

package desktop

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv/desktop/rdpstate/rdpstatetest"
	"github.com/gravitational/teleport/lib/utils/testutils"
)

func TestProperty_DesktopProcessor_RandomTimelineNeverPanics(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(0, 30).Draw(t, "count")
		base := time.Unix(1_700_000_000, 0).UTC()

		events := make([]apievents.AuditEvent, 0, count+2)
		events = append(events, &apievents.WindowsDesktopSessionStart{
			Metadata: apievents.Metadata{Time: base},
		})

		hello, err := rdpstatetest.EncodeTDPBServerHello(400, 300)
		require.NoError(t, err)

		hello.Metadata = apievents.Metadata{Time: base.Add(50 * time.Millisecond)}
		events = append(events, hello)

		cursor := base.Add(100 * time.Millisecond)
		for range count {
			deltaMs := rapid.Int64Range(0, 5_000).Draw(t, "delta")
			cursor = cursor.Add(time.Duration(deltaMs) * time.Millisecond)

			kind := rapid.IntRange(0, 2).Draw(t, "kind")
			var pdu []byte
			switch kind {
			case 0:
				left := rapid.IntRange(0, 396).Draw(t, "left")
				top := rapid.IntRange(0, 296).Draw(t, "top")
				w := rapid.IntRange(4, 400-left).Draw(t, "w")
				h := rapid.IntRange(1, 300-top).Draw(t, "h")
				pdu = rdpstatetest.BuildBitmapPDU(left, top, w, h, rdpstatetest.RGB565White)

			case 1:
				pdu = rdpstatetest.BuildPointerPositionPDU(
					rapid.IntRange(0, 399).Draw(t, "px"),
					rapid.IntRange(0, 299).Draw(t, "py"),
				)

			case 2:
				pdu = rdpstatetest.BuildPointerHiddenPDU()
			}

			evt, err := rdpstatetest.EncodeTDPBFastPathPDU(pdu)
			require.NoError(t, err)

			evt.Metadata = apievents.Metadata{Time: cursor}
			events = append(events, evt)
		}

		testutils.RunWithTimeout(t, 5*time.Second, func() {
			d := NewRecordingProcessor(NewGlyphCache())
			defer d.Release()

			for _, evt := range events {
				if _, err := d.ProcessEvent(evt); err != nil {
					// Errors are acceptable, we're ensuring there are no panics.
				}
			}
			_, _ = d.Flush()
		})
	})
}

func TestProperty_DesktopProcessor_ScreenshotsAreWellFormed(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(1, 20).Draw(t, "count")
		base := time.Unix(1_700_000_000, 0).UTC()

		events := []apievents.AuditEvent{
			&apievents.WindowsDesktopSessionStart{Metadata: apievents.Metadata{Time: base}},
		}
		hello, err := rdpstatetest.EncodeTDPBServerHello(800, 600)
		require.NoError(t, err)

		hello.Metadata = apievents.Metadata{Time: base.Add(50 * time.Millisecond)}
		events = append(events, hello)

		cursor := base.Add(2 * time.Second)
		for range count {
			cursor = cursor.Add(2 * time.Second)
			pdu := rdpstatetest.BuildBitmapPDU(
				rapid.IntRange(0, 700).Draw(t, "left"),
				rapid.IntRange(0, 500).Draw(t, "top"),
				rapid.IntRange(4, 100).Draw(t, "w"),
				rapid.IntRange(1, 100).Draw(t, "h"),
				rdpstatetest.RGB565White,
			)

			evt, err := rdpstatetest.EncodeTDPBFastPathPDU(pdu)
			require.NoError(t, err)

			evt.Metadata = apievents.Metadata{Time: cursor}
			events = append(events, evt)
		}

		d := NewRecordingProcessor(NewGlyphCache())
		defer d.Release()

		var emitted []ScreenshotResult
		for _, evt := range events {
			res, err := d.ProcessEvent(evt)
			if err != nil {
				// Errors are acceptable - we only assert on emitted screenshots.
				continue
			}
			if res != nil {
				emitted = append(emitted, *res)
			}
		}

		flushed, err := d.Flush()
		require.NoError(t, err)
		emitted = append(emitted, flushed...)

		for i, sr := range emitted {
			require.NotEmpty(t, sr.PNG, "result %d has empty PNG", i)
			require.LessOrEqual(t, len(sr.PNG), screenshotMaxPixels*4,
				"result %d PNG too large (%d bytes)", i, len(sr.PNG))
		}

		for i := 1; i < len(emitted); i++ {
			gap := emitted[i].StartTime - emitted[i-1].StartTime
			require.GreaterOrEqual(t, gap, time.Duration(0),
				"emitted[%d].StartTime %v before emitted[%d] %v",
				i, emitted[i].StartTime, i-1, emitted[i-1].StartTime)
		}
	})
}

func TestDesktopProcessor_FlushAfterReleaseNeverPanics(t *testing.T) {
	base := time.Unix(1_700_000_000, 0).UTC()

	testutils.RunWithTimeout(t, 1*time.Second, func() {
		d := NewRecordingProcessor(NewGlyphCache())
		_, _ = d.ProcessEvent(&apievents.WindowsDesktopSessionStart{
			Metadata: apievents.Metadata{Time: base},
		})
		d.Release()

		// Subsequent Flush / ProcessEvent must not panic post-Release.
		_, _ = d.Flush()
		_, _ = d.ProcessEvent(&apievents.WindowsDesktopSessionStart{
			Metadata: apievents.Metadata{Time: base.Add(time.Second)},
		})
	})
}
