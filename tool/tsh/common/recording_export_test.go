/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/desktop/rdpstate/rdpstatetest"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/legacy"
	"github.com/gravitational/teleport/tool/tsh/common/internal/recordingexport"
)

var pngFrame []byte

func init() {
	pngFrame, _ = base64.StdEncoding.DecodeString("GwAAAkUAAAGAAAABQAAAAb8AAAF/iVBORw0KGgoAAAANSUhEUgAAAEAAAABACAYAAACqaXHeAAACDElEQVR4Ae1WLU8DQRCd/oPisODqaBVpAwJHSRBtEKSCALJNMCAIlU0QYEhaCQmiimBIKA4BaXFAgqijhgQcJxC4wtv0Ntvl4A4FvX2X7O3OzX7MzM28twlJF/rt433Bk1vZRKea+Q1jz/NkYaOmdObr8awurZuuVHYb5mcx19S3y1Kaz8rYbEnPKS/npb61LolMUfzn/KAq+Zm0Lw71mGfuOaQcCK9XTWledL7YMlAHdv8uAMlkUswfYVr9ZwHAn0lNjMvkYsW0R41hFAa20ebfiJoBQfOwt99+sgNzzDMhR2mRMgAb9W9PpXV9p8sAxqAkslMpaR/VpHFyqVMPumw6pVM+yLGgEvDP6Tz0dBZg/53Voj73OzuwFuXY7b3oubADJYXygT6oRQ4AFuNw9Gi9p2edETASQcB3NO/tXTsP+TcBwHzzHMi5tap07rsYqmbqTTtwTnlpTs2p7B1KfjqjMCU0AGqFoy+VAY76rtxmAHAPUKFw9MUMYAZ8XoUdzX7lNkuAJcASKPRVMTjyst0kBhADiAHEAIKgjYwuyWQBsgBZgCxAFnAJ9W1fyQJkAbJAvFnArnlbJgYQA4gBxADeBG1kdEkmC5AFyAJkAbKAS6hv+0oWIAuQBeLFAnaNh8nEAGIAMYAYwJtgGFLGWU8WIAuQBcgCZIE4o3yYb2QBsgBZYLRZIKzGw/QfdfZB0MXQrqAAAAAASUVORK5CYII=")

	binary.BigEndian.PutUint32(pngFrame[5:], 0)   // left
	binary.BigEndian.PutUint32(pngFrame[9:], 0)   // top
	binary.BigEndian.PutUint32(pngFrame[13:], 64) // right
	binary.BigEndian.PutUint32(pngFrame[17:], 64) // bottom
}

func TestUnsupportedRecordings(t *testing.T) {
	t.Parallel()

	for _, startEvent := range []apievents.AuditEvent{
		new(apievents.SessionStart),
		new(apievents.AppSessionStart),
		new(apievents.MCPSessionStart),
		new(apievents.DatabaseSessionStart),
	} {
		t.Run(fmt.Sprintf("%T", startEvent), func(t *testing.T) {
			events := []apievents.AuditEvent{startEvent}
			exporter := &desktopRecordingExporter{
				ss:  eventstest.NewFakeStreamer(events, 0),
				sid: session.NewID(),
			}
			_, err := exporter.getSessionMetadata(t.Context())
			require.Error(t, err)
			require.True(t, trace.IsBadParameter(err), "expected bad paramater error, got %v", err)
		})
	}
}

func TestGetSessionMetadata(t *testing.T) {
	t.Parallel()

	events := []apievents.AuditEvent{
		&apievents.WindowsDesktopSessionStart{},
		tdpEvent(t, legacy.ClientScreenSpec{Width: 1024, Height: 768}),
		tdpEvent(t, legacy.PNG2Frame(pngFrame)),
		tdpEvent(t, legacy.PNG2Frame(pngFrame)),
		tdpEvent(t, legacy.ClientScreenSpec{Width: 1920, Height: 1080}),
		tdpEvent(t, legacy.PNG2Frame(pngFrame)),
		&apievents.WindowsDesktopSessionEnd{},
	}

	exporter := &desktopRecordingExporter{
		ss:  eventstest.NewFakeStreamer(events, 0),
		sid: session.NewID(),
	}

	meta, err := exporter.getSessionMetadata(t.Context())
	require.NoError(t, err)
	require.NotNil(t, meta)

	assert.False(t, meta.isRemoteFX)
	assert.EqualValues(t, 1920, meta.maxWidth)
	assert.EqualValues(t, 1080, meta.maxHeight)
	assert.EqualValues(t, len(events), meta.totalEvents)
}

func TestWritesOneFrame(t *testing.T) {
	t.Parallel()

	oneFrame := frameDelayMillis
	// need a PNG that will actually decode
	events := []apievents.AuditEvent{
		tdpEventMillis(t, legacy.ClientScreenSpec{Width: 128, Height: 128}, 0),
		tdpEventMillis(t, legacy.PNG2Frame(pngFrame), 0),
		tdpEventMillis(t, legacy.PNG2Frame(pngFrame), int64(oneFrame)+1),
	}

	exporter := &desktopRecordingExporter{
		ss:  eventstest.NewFakeStreamer(events, 0),
		sid: session.NewID(),
	}

	frames, err := exporter.run(
		t.Context(),
		&recordingMetadata{},
		recordingexport.NewPNGDecoder(128, 128),
		nopEncoder{})
	require.NoError(t, err)
	require.Equal(t, 1, frames)
}

func TestCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	events := []apievents.AuditEvent{&apievents.WindowsDesktopSessionStart{}}
	exporter := &desktopRecordingExporter{
		ss:  eventstest.NewFakeStreamer(events, 3*time.Second),
		sid: session.NewID(),
	}

	frames, err := exporter.run(
		ctx,
		&recordingMetadata{},
		recordingexport.NewPNGDecoder(128, 128),
		nopEncoder{})
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 0, frames)
}

func TestWritesManyFrames(t *testing.T) {
	t.Parallel()

	events := []apievents.AuditEvent{
		tdpEventMillis(t, legacy.ClientScreenSpec{Width: 128, Height: 128}, 0),
		tdpEventMillis(t, legacy.PNG2Frame(pngFrame), 0),

		// the final frame is just over 1s after the first frame
		tdpEventMillis(t, legacy.PNG2Frame(pngFrame), 1001),
	}

	exporter := &desktopRecordingExporter{
		ss:  eventstest.NewFakeStreamer(events, 0),
		sid: session.NewID(),
	}

	frames, err := exporter.run(
		t.Context(),
		&recordingMetadata{},
		recordingexport.NewPNGDecoder(128, 128),
		nopEncoder{})
	require.NoError(t, err)
	require.Equal(t, framesPerSecond, frames)
}

func TestGetSessionMetadataRemoteFX(t *testing.T) {
	t.Parallel()

	legacyActivated, err := rdpstatetest.LegacyConnectionActivated(1280, 720)
	require.NoError(t, err)
	tdpbHello, err := rdpstatetest.EncodeTDPBServerHello(1600, 900)
	require.NoError(t, err)
	tdpbFrame, err := rdpstatetest.EncodeTDPBFastPathPDU([]byte{0x00})
	require.NoError(t, err)

	for _, tc := range []struct {
		name          string
		events        []apievents.AuditEvent
		width, height uint32
	}{
		{
			name: "legacy",
			events: []apievents.AuditEvent{
				&apievents.WindowsDesktopSessionStart{},
				legacyActivated,
				tdpEvent(t, legacy.RDPFastPathPDU([]byte{0x00})),
				&apievents.WindowsDesktopSessionEnd{},
			},
			width:  1280,
			height: 720,
		},
		{
			name: "tdpb",
			events: []apievents.AuditEvent{
				&apievents.WindowsDesktopSessionStart{},
				tdpbHello,
				tdpbFrame,
				&apievents.WindowsDesktopSessionEnd{},
			},
			width:  1600,
			height: 900,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exporter := &desktopRecordingExporter{
				ss:  eventstest.NewFakeStreamer(tc.events, 0),
				sid: session.NewID(),
			}

			meta, err := exporter.getSessionMetadata(t.Context())
			require.NoError(t, err)
			require.NotNil(t, meta)

			assert.True(t, meta.isRemoteFX)
			assert.EqualValues(t, tc.width, meta.maxWidth)
			assert.EqualValues(t, tc.height, meta.maxHeight)
			assert.EqualValues(t, len(tc.events), meta.totalEvents)
		})
	}
}

func TestExportEmitsConsistentFrameSize(t *testing.T) {
	t.Parallel()

	events := []apievents.AuditEvent{
		tdpEventMillis(t, legacy.RDPFastPathPDU([]byte{0}), 0),
		tdpEventMillis(t, legacy.RDPFastPathPDU([]byte{0}), 50),
		tdpEventMillis(t, legacy.RDPFastPathPDU([]byte{0}), 100),
	}

	dec := &resizingDecoder{sizes: []image.Rectangle{
		image.Rect(0, 0, 64, 64),
		image.Rect(0, 0, 64, 64),
		image.Rect(0, 0, 128, 128),
	}}
	enc := &sizeCapturingEncoder{}

	exporter := &desktopRecordingExporter{
		ss:  eventstest.NewFakeStreamer(events, 0),
		sid: session.NewID(),
	}

	_, err := exporter.run(t.Context(), &recordingMetadata{maxWidth: 128, maxHeight: 128}, dec, enc)
	require.NoError(t, err)
	require.NotEmpty(t, enc.sizes)

	for _, s := range enc.sizes {
		assert.Equal(t, image.Pt(128, 128), s, "every emitted frame must be the recording's max size")
	}
}

func tdpEvent(t *testing.T, msg tdp.Message) *apievents.DesktopRecording {
	t.Helper()

	b, err := msg.Encode()
	require.NoError(t, err)

	return &apievents.DesktopRecording{Message: b}
}

func tdpEventMillis(t *testing.T, msg tdp.Message, millis int64) *apievents.DesktopRecording {
	t.Helper()

	evt := tdpEvent(t, msg)
	evt.DelayMilliseconds = millis
	return evt
}

type nopEncoder struct{}

func (nopEncoder) EmitFrames(image.Image, int) error { return nil }
func (nopEncoder) OutputFiles() []string             { return nil }
func (nopEncoder) Close() error                      { return nil }

func TestReportOutputFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		files    []string
		contains []string
	}{
		{
			name:     "no files",
			files:    nil,
			contains: []string{"No video was written"},
		},
		{
			name:     "single file",
			files:    []string{"session.avi"},
			contains: []string{"Wrote recording to session.avi"},
		},
		{
			name:     "multiple files",
			files:    []string{"session.avi", "session-1.avi", "session-2.avi"},
			contains: []string{"3 files", "session.avi", "session-1.avi", "session-2.avi"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var buf bytes.Buffer
			reportOutputFiles(&buf, test.files)
			for _, want := range test.contains {
				require.Contains(t, buf.String(), want)
			}
		})
	}
}

type resizingDecoder struct {
	sizes []image.Rectangle
	i     int
	cur   image.Rectangle
}

func (d *resizingDecoder) UpdateScreen(*apievents.DesktopRecording) (bool, error) {
	if d.i < len(d.sizes) {
		d.cur = d.sizes[d.i]
		d.i++
	}
	return true, nil
}

func (d *resizingDecoder) Image() image.Image {
	if d.cur.Empty() {
		return nil
	}
	return image.NewRGBA(d.cur)
}

func (d *resizingDecoder) Close() error { return nil }

type sizeCapturingEncoder struct {
	sizes []image.Point
}

func (e *sizeCapturingEncoder) EmitFrames(img image.Image, count int) error {
	b := img.Bounds()
	for range count {
		e.sizes = append(e.sizes, image.Pt(b.Dx(), b.Dy()))
	}
	return nil
}

func (e *sizeCapturingEncoder) OutputFiles() []string { return nil }
func (e *sizeCapturingEncoder) Close() error          { return nil }
