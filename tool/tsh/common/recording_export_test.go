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
	"context"
	"encoding/base64"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
)

var pngFrame []byte

func init() {
	pngFrame, _ = base64.StdEncoding.DecodeString("GwAAAkUAAAGAAAABQAAAAb8AAAF/iVBORw0KGgoAAAANSUhEUgAAAEAAAABACAYAAACqaXHeAAACDElEQVR4Ae1WLU8DQRCd/oPisODqaBVpAwJHSRBtEKSCALJNMCAIlU0QYEhaCQmiimBIKA4BaXFAgqijhgQcJxC4wtv0Ntvl4A4FvX2X7O3OzX7MzM28twlJF/rt433Bk1vZRKea+Q1jz/NkYaOmdObr8awurZuuVHYb5mcx19S3y1Kaz8rYbEnPKS/npb61LolMUfzn/KAq+Zm0Lw71mGfuOaQcCK9XTWledL7YMlAHdv8uAMlkUswfYVr9ZwHAn0lNjMvkYsW0R41hFAa20ebfiJoBQfOwt99+sgNzzDMhR2mRMgAb9W9PpXV9p8sAxqAkslMpaR/VpHFyqVMPumw6pVM+yLGgEvDP6Tz0dBZg/53Voj73OzuwFuXY7b3oubADJYXygT6oRQ4AFuNw9Gi9p2edETASQcB3NO/tXTsP+TcBwHzzHMi5tap07rsYqmbqTTtwTnlpTs2p7B1KfjqjMCU0AGqFoy+VAY76rtxmAHAPUKFw9MUMYAZ8XoUdzX7lNkuAJcASKPRVMTjyst0kBhADiAHEAIKgjYwuyWQBsgBZgCxAFnAJ9W1fyQJkAbJAvFnArnlbJgYQA4gBxADeBG1kdEkmC5AFyAJkAbKAS6hv+0oWIAuQBeLFAnaNh8nEAGIAMYAYwJtgGFLGWU8WIAuQBcgCZIE4o3yYb2QBsgBZYLRZIKzGw/QfdfZB0MXQrqAAAAAASUVORK5CYII=")

	binary.BigEndian.PutUint32(pngFrame[5:], 0)   // left
	binary.BigEndian.PutUint32(pngFrame[9:], 0)   // top
	binary.BigEndian.PutUint32(pngFrame[13:], 64) // right
	binary.BigEndian.PutUint32(pngFrame[17:], 64) // bottom
}

func TestWriteMovieCanBeCanceled(t *testing.T) {
	t.Parallel()

	events := []apievents.AuditEvent{
		&apievents.WindowsDesktopSessionStart{},
	}
	fs := eventstest.NewFakeStreamer(events, 3*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	frames, _, err := writeMovieWrapper(t, ctx, fs, "test", "test", nil)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 0, frames)
}

func TestWriteMovieDoesNotSupportSSH(t *testing.T) {
	t.Parallel()

	events := []apievents.AuditEvent{
		&apievents.SessionStart{},
	}
	fs := eventstest.NewFakeStreamer(events, 0)

	frames, _, err := writeMovieWrapper(t, context.Background(), fs, "test", "test", nil)
	require.True(t, trace.IsBadParameter(err), "expected bad paramater error, got %v", err)
	require.Equal(t, 0, frames)
}

// TestWriteMovieMultipleScreenSpecs verifies that the export fails when a session recording
// contains multiple TDP screen spec messages.
//
// At the time of this implementation, desktop access does not support resizing during the
// screen during a session. This test exists to prevent regressions should that behavior change.
func TestWriteMovieMultipleScreenSpecs(t *testing.T) {
	t.Parallel()

	events := []apievents.AuditEvent{
		tdpEvent(t, tdp.ClientScreenSpec{Width: 1920, Height: 1080}),
		tdpEvent(t, tdp.ClientScreenSpec{Width: 1920, Height: 1080}),
	}

	fs := eventstest.NewFakeStreamer(events, 0)
	frames, _, err := writeMovieWrapper(t, context.Background(), fs, session.ID("test"), "test", nil)
	require.True(t, trace.IsBadParameter(err), "expected bad paramater error, got %v", err)
	require.Equal(t, 0, frames)
}

func TestWriteMovieWritesOneFrame(t *testing.T) {
	t.Parallel()

	oneFrame := frameDelayMillis
	// need a PNG that will actually decode
	events := []apievents.AuditEvent{
		tdpEventMillis(t, tdp.ClientScreenSpec{Width: 128, Height: 128}, 0),
		tdpEventMillis(t, tdp.PNG2Frame(pngFrame), 0),
		tdpEventMillis(t, tdp.PNG2Frame(pngFrame), int64(oneFrame)+1),
	}
	fs := eventstest.NewFakeStreamer(events, 0)
	frames, _, err := writeMovieWrapper(t, context.Background(), fs, session.ID("test"), "test", nil)
	require.NoError(t, err)
	require.Equal(t, 1, frames)
}

func TestWriteMovieWritesManyFrames(t *testing.T) {
	t.Parallel()

	events := []apievents.AuditEvent{
		tdpEventMillis(t, tdp.ClientScreenSpec{Width: 128, Height: 128}, 0),
		tdpEventMillis(t, tdp.PNG2Frame(pngFrame), 0),

		// the final frame is just over 1s after the first frame
		tdpEventMillis(t, tdp.PNG2Frame(pngFrame), 1001),
	}
	fs := eventstest.NewFakeStreamer(events, 0)
	t.Cleanup(func() { os.RemoveAll("test.avi") })
	frames, _, err := writeMovieWrapper(t, context.Background(), fs, session.ID("test"), "test", nil)
	require.NoError(t, err)
	require.Equal(t, framesPerSecond, frames)
}

// writeMovieWrapper calls writeMovie, and tells the test state to cleanup the created files upon completion.
// Returns the writeMovie call results, as well as the path-qualified prefix to the created file.
func writeMovieWrapper(t *testing.T, ctx context.Context, ss events.SessionStreamer, sid session.ID, prefix string,
	write func(format string, args ...any) (int, error)) (int, string, error) {

	tempDir := t.TempDir()
	prefix = filepath.Join(tempDir, prefix)
	frames, err := writeMovie(ctx, ss, sid, prefix, write, "")
	return frames, prefix, err
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
