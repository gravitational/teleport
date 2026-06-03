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
	"encoding/json"
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
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/legacy"
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
		tdpEvent(t, legacy.ClientScreenSpec{Width: 1920, Height: 1080}),
		tdpEvent(t, legacy.ClientScreenSpec{Width: 1920, Height: 1080}),
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
		tdpEventMillis(t, legacy.ClientScreenSpec{Width: 128, Height: 128}, 0),
		tdpEventMillis(t, legacy.PNG2Frame(pngFrame), 0),
		tdpEventMillis(t, legacy.PNG2Frame(pngFrame), int64(oneFrame)+1),
	}
	fs := eventstest.NewFakeStreamer(events, 0)
	frames, _, err := writeMovieWrapper(t, context.Background(), fs, session.ID("test"), "test", nil)
	require.NoError(t, err)
	require.Equal(t, 1, frames)
}

func TestWriteMovieWritesManyFrames(t *testing.T) {
	t.Parallel()

	events := []apievents.AuditEvent{
		tdpEventMillis(t, legacy.ClientScreenSpec{Width: 128, Height: 128}, 0),
		tdpEventMillis(t, legacy.PNG2Frame(pngFrame), 0),

		// the final frame is just over 1s after the first frame
		tdpEventMillis(t, legacy.PNG2Frame(pngFrame), 1001),
	}
	fs := eventstest.NewFakeStreamer(events, 0)
	t.Cleanup(func() { os.RemoveAll("test.avi") })
	frames, _, err := writeMovieWrapper(t, context.Background(), fs, session.ID("test"), "test", nil)
	require.NoError(t, err)
	require.Equal(t, framesPerSecond, frames)
}

// writeMovieWrapper creates a stream from ss and calls writeMovie.
// Returns the frame count, the path-qualified prefix, and any error.
func writeMovieWrapper(t *testing.T, ctx context.Context, ss events.SessionStreamer, sid session.ID, prefix string,
	write func(format string, args ...any) (int, error)) (int, string, error) {

	evts, errs := ss.StreamSessionEvents(ctx, sid, 0)
	tempDir := t.TempDir()
	prefix = filepath.Join(tempDir, prefix)
	frames, err := writeMovie(ctx, evts, errs, prefix, write, "")
	return frames, prefix, err
}

// writeHARWrapper creates a stream from ss and calls writeHAR.
// Returns the output path and any error.
func writeHARWrapper(t *testing.T, ctx context.Context, ss events.SessionStreamer, sid session.ID,
	write func(format string, args ...any) (int, error)) (string, error) {

	evts, errs := ss.StreamSessionEvents(ctx, sid, 0)
	outputPath := filepath.Join(t.TempDir(), "session.har")
	return outputPath, writeHAR(ctx, evts, errs, outputPath, write)
}

func TestWriteHAR(t *testing.T) {
	t.Parallel()

	reqTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	requestID := "req-1"

	evts := []apievents.AuditEvent{
		&apievents.AppSessionStart{},
		&apievents.AppSessionHTTPRequest{
			Metadata: apievents.Metadata{
				Type: "app.session.http.request",
				Time: reqTime,
			},
			RequestId:   requestID,
			Method:      "POST",
			Url:         "https://app.example.com/api?q=1",
			HttpVersion: "HTTP/1.1",
			Headers: []*apievents.HTTPHeader{
				{Name: "Content-Type", Value: "application/json"},
			},
			RawQuery: "q=1",
		},
		&apievents.AppSessionHTTPRequestBodyChunk{
			RequestId:  requestID,
			ChunkIndex: 0,
			IsLast:     false,
			Data:       []byte(`{"key`),
		},
		&apievents.AppSessionHTTPRequestBodyChunk{
			RequestId:  requestID,
			ChunkIndex: 1,
			IsLast:     true,
			Data:       []byte(`":"val"}`),
		},
		&apievents.AppSessionHTTPResponse{
			RequestId:   requestID,
			StatusCode:  200,
			StatusText:  "200 OK",
			HttpVersion: "HTTP/1.1",
			Headers: []*apievents.HTTPHeader{
				{Name: "Content-Type", Value: "application/json"},
			},
			WaitTimeMs: 42,
		},
		&apievents.AppSessionHTTPResponseBodyChunk{
			RequestId:  requestID,
			ChunkIndex: 0,
			IsLast:     true,
			Data:       []byte(`{"ok":true}`),
		},
	}

	fs := eventstest.NewFakeStreamer(evts, 0)
	outputPath, err := writeHARWrapper(t, context.Background(), fs, session.ID("test"), nil)
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	var result harRoot
	require.NoError(t, json.Unmarshal(data, &result))

	require.Equal(t, "1.2", result.Log.Version)
	require.Len(t, result.Log.Entries, 1)

	entry := result.Log.Entries[0]
	require.Equal(t, "2026-01-02T03:04:05.000Z", entry.StartedDateTime)
	require.Equal(t, float64(42), entry.Time) // wait only (send+receive are 0)

	req := entry.Request
	require.Equal(t, "POST", req.Method)
	require.Equal(t, "https://app.example.com/api?q=1", req.URL)
	require.Equal(t, "HTTP/1.1", req.HTTPVersion)
	require.NotNil(t, req.PostData)
	require.Equal(t, "application/json", req.PostData.MimeType)
	require.Equal(t, `{"key":"val"}`, req.PostData.Text)
	require.Empty(t, req.PostData.Encoding) // JSON is text
	require.Len(t, req.QueryString, 1)
	require.Equal(t, "q", req.QueryString[0].Name)
	require.Equal(t, "1", req.QueryString[0].Value)

	resp := entry.Response
	require.Equal(t, 200, resp.Status)
	require.Equal(t, "OK", resp.StatusText)
	require.Equal(t, "application/json", resp.Content.MimeType)
	require.Equal(t, `{"ok":true}`, resp.Content.Text)
	require.Empty(t, resp.Content.Encoding)

	require.Equal(t, float64(42), entry.Timings.Wait)
}

func TestWriteHAR_BinaryBody(t *testing.T) {
	t.Parallel()

	requestID := "req-bin"
	evts := []apievents.AuditEvent{
		&apievents.AppSessionStart{},
		&apievents.AppSessionHTTPRequest{
			Metadata:    apievents.Metadata{Type: "app.session.http.request"},
			RequestId:   requestID,
			Method:      "GET",
			Url:         "https://app.example.com/img.png",
			HttpVersion: "HTTP/1.1",
		},
		&apievents.AppSessionHTTPResponse{
			RequestId:   requestID,
			StatusCode:  200,
			StatusText:  "200 OK",
			HttpVersion: "HTTP/1.1",
			Headers: []*apievents.HTTPHeader{
				{Name: "Content-Type", Value: "image/png"},
			},
		},
		&apievents.AppSessionHTTPResponseBodyChunk{
			RequestId:  requestID,
			ChunkIndex: 0,
			IsLast:     true,
			Data:       []byte{0x89, 0x50, 0x4E, 0x47}, // PNG magic bytes
		},
	}

	fs := eventstest.NewFakeStreamer(evts, 0)
	outputPath, err := writeHARWrapper(t, context.Background(), fs, session.ID("test"), nil)
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	var result harRoot
	require.NoError(t, json.Unmarshal(data, &result))

	resp := result.Log.Entries[0].Response
	require.Equal(t, "base64", resp.Content.Encoding)
	require.Equal(t, "iVBORw==", resp.Content.Text) // base64 of the PNG magic bytes
}

func TestWriteMovieDetectsAppSession(t *testing.T) {
	t.Parallel()

	evts := []apievents.AuditEvent{
		&apievents.AppSessionStart{},
	}
	fs := eventstest.NewFakeStreamer(evts, 0)

	_, _, err := writeMovieWrapper(t, context.Background(), fs, session.ID("test"), "test", nil)
	require.True(t, trace.IsBadParameter(err), "expected bad parameter error, got %v", err)
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
