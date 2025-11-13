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

package filesessions

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"iter"
	mathrand "math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/session"
)

// TestUploadOK tests async file uploads scenarios
func TestUploadOK(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	p := newUploaderPack(ctx, t, uploaderPackConfig{})

	// wait until uploader blocks on the clock
	p.clock.BlockUntilContext(ctx, 1)

	inEvents := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 1024})
	sid := inEvents[0].(events.SessionMetadataGetter).GetSessionID()

	p.emitEvents(ctx, t, inEvents)

	// initiate the scan by advancing clock past
	// block period
	p.clock.Advance(p.scanPeriod + time.Second)

	var event events.UploadEvent
	select {
	case event = <-p.memEventsC:
		require.Equal(t, event.SessionID, sid)
		require.NoError(t, event.Error)
	case <-ctx.Done():
		t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
	}

	// read the upload and make sure the data is equal
	outEvents := p.readEvents(ctx, t, event.UploadID)
	require.Equal(t, inEvents, outEvents)

	// regression: ensure there is a single upload for the captured session
	uploads, err := p.memUploader.ListUploads(ctx)
	require.NoError(t, err)
	require.Len(t, uploads, 1)
}

// TestUploadParallel verifies several parallel uploads that have to wait
// for semaphore
func TestUploadParallel(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	p := newUploaderPack(ctx, t, uploaderPackConfig{})

	// wait until uploader blocks on the clock
	p.clock.BlockUntilContext(ctx, 1)

	sessions := make(map[string][]apievents.AuditEvent)

	const sessionCount = 5
	for range sessionCount {
		sessionEvents := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 1024})
		sid := sessionEvents[0].(events.SessionMetadataGetter).GetSessionID()

		p.emitEvents(ctx, t, sessionEvents)
		sessions[sid] = sessionEvents
	}

	// initiate the scan by advancing the clock past
	// block period
	p.clock.Advance(p.scanPeriod + time.Second)

	for range sessions {
		var event events.UploadEvent
		var sessionEvents []apievents.AuditEvent
		var found bool
		select {
		case event = <-p.memEventsC:
			require.NoError(t, event.Error)
			sessionEvents, found = sessions[event.SessionID]
			require.True(t, found, "session %q is not expected, possible duplicate event", event.SessionID)
		case <-ctx.Done():
			t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
		}

		// read the upload and make sure the data is equal
		outEvents := p.readEvents(ctx, t, event.UploadID)

		require.Equal(t, sessionEvents, outEvents)

		delete(sessions, event.SessionID)
	}

	// regression: ensure there is a single upload for each captured session
	uploads, err := p.memUploader.ListUploads(ctx)
	require.NoError(t, err)
	require.Len(t, uploads, sessionCount)
}

// TestMovesCorruptedUploads verifies that the uploader moves corrupted uploads
// out of the scan directory.
func TestMovesCorruptedUploads(t *testing.T) {
	scanDir := t.TempDir()
	corruptedDir := t.TempDir()

	uploader, err := NewUploader(UploaderConfig{
		Streamer:     events.NewDiscardStreamer(),
		ScanDir:      scanDir,
		CorruptedDir: corruptedDir,
	})
	require.NoError(t, err)

	sessionID := session.NewID()
	uploadPath := filepath.Join(scanDir, sessionID.String()+".tar")
	errorPath := filepath.Join(scanDir, sessionID.String()+".error")
	badFilePath := filepath.Join(scanDir, "not-a-uuid.tar")

	// create a "corrupted" upload and error file in the scan dir
	b := make([]byte, 4096)
	rand.Read(b)
	require.NoError(t, os.WriteFile(uploadPath, b, 0o600))
	require.NoError(t, uploader.writeSessionError(sessionID, errors.New("this is a corrupted upload")))

	// create a file with an invalid name (not a session ID)
	badFile, err := os.Create(badFilePath)
	require.NoError(t, err)
	require.NoError(t, badFile.Close())

	stats, err := uploader.Scan(t.Context())
	require.NoError(t, err)
	require.Equal(t, 2, stats.Scanned)
	require.Equal(t, 2, stats.Corrupted)
	require.Equal(t, 0, stats.Started)

	require.NoFileExists(t, uploadPath)
	require.NoFileExists(t, errorPath)

	require.FileExists(t, filepath.Join(corruptedDir, filepath.Base(uploadPath)))
	require.FileExists(t, filepath.Join(corruptedDir, filepath.Base(errorPath)))

	// run a second scan to verify that:
	// 1. the corrupted file is no longer processed
	// 2. the file with the bad name was still flagged as corrupted
	stats, err = uploader.Scan(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, stats.Scanned)
	require.Equal(t, 1, stats.Corrupted)
	require.Equal(t, 0, stats.Started)
}

type resumeTestCase struct {
	name    string
	newTest func(streamer events.Streamer) resumeTestTuple
	// retries is how many times the uploader will retry the upload
	// after the first upload attempt fails
	retries int
	// onRetry is called on retry attempt
	onRetry func(t *testing.T, attempt int, uploader *Uploader)
}

type resumeTestTuple struct {
	streamer *events.CallbackStreamer
	verify   func(t *testing.T, tc resumeTestCase)
}

// TestUploadResume verifies successful upload run after the stream has been interrupted
func TestUploadResume(t *testing.T) {
	testCases := []resumeTestCase{
		{
			name:    "stream terminates in the middle of submission",
			retries: 1,
			newTest: func(streamer events.Streamer) resumeTestTuple {
				var streamResumed, terminateConnection atomic.Uint64
				terminateConnection.Store(1)

				callbackStreamer, err := events.NewCallbackStreamer(events.CallbackStreamerConfig{
					Inner: streamer,
					OnRecordEvent: func(ctx context.Context, sid session.ID, pe apievents.PreparedSessionEvent) error {
						event := pe.GetAuditEvent()
						if event.GetIndex() > 600 && terminateConnection.CompareAndSwap(1, 0) == true {
							t.Logf("Terminating connection at event %v", event.GetIndex())
							return trace.ConnectionProblem(nil, "connection terminated")
						}
						return nil
					},
					OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer events.Streamer) (apievents.Stream, error) {
						stream, err := streamer.ResumeAuditStream(ctx, sid, uploadID)
						require.NoError(t, err)
						streamResumed.Add(1)
						return stream, nil
					},
				})
				require.NoError(t, err)
				return resumeTestTuple{
					streamer: callbackStreamer,
					verify: func(t *testing.T, tc resumeTestCase) {
						require.Equal(t, 1, int(streamResumed.Load()), tc.name)
					},
				}
			},
		},
		{
			name:    "stream terminates multiple times at different stages of submission",
			retries: 10,
			newTest: func(streamer events.Streamer) resumeTestTuple {
				var streamResumed, terminateConnection atomic.Uint64

				callbackStreamer, err := events.NewCallbackStreamer(events.CallbackStreamerConfig{
					Inner: streamer,
					OnRecordEvent: func(ctx context.Context, sid session.ID, pe apievents.PreparedSessionEvent) error {
						event := pe.GetAuditEvent()
						if event.GetIndex() > 600 && terminateConnection.Add(1) <= 10 {
							t.Logf("Terminating connection #%v at event %v", terminateConnection.Load(), event.GetIndex())
							return trace.ConnectionProblem(nil, "connection terminated")
						}
						return nil
					},
					OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer events.Streamer) (apievents.Stream, error) {
						stream, err := streamer.ResumeAuditStream(ctx, sid, uploadID)
						require.NoError(t, err)
						streamResumed.Add(1)
						return stream, nil
					},
				})
				require.NoError(t, err)
				return resumeTestTuple{
					streamer: callbackStreamer,
					verify: func(t *testing.T, tc resumeTestCase) {
						require.Equal(t, 10, int(streamResumed.Load()), tc.name)
					},
				}
			},
		},
		{
			name:    "stream resumes if upload is not found",
			retries: 1,
			newTest: func(streamer events.Streamer) resumeTestTuple {
				var streamCreated, terminateConnection atomic.Uint64
				terminateConnection.Store(1)

				callbackStreamer, err := events.NewCallbackStreamer(events.CallbackStreamerConfig{
					Inner: streamer,
					OnRecordEvent: func(ctx context.Context, sid session.ID, pe apievents.PreparedSessionEvent) error {
						event := pe.GetAuditEvent()
						if event.GetIndex() > 600 && terminateConnection.CompareAndSwap(1, 0) == true {
							t.Logf("Terminating connection at event %v", event.GetIndex())
							return trace.ConnectionProblem(nil, "connection terminated")
						}
						return nil
					},
					OnCreateAuditStream: func(ctx context.Context, sid session.ID, streamer events.Streamer) (apievents.Stream, error) {
						stream, err := streamer.CreateAuditStream(ctx, sid)
						require.NoError(t, err)
						streamCreated.Add(1)
						return stream, nil
					},
					OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer events.Streamer) (apievents.Stream, error) {
						return nil, trace.NotFound("stream not found")
					},
				})
				require.NoError(t, err)
				return resumeTestTuple{
					streamer: callbackStreamer,
					verify: func(t *testing.T, tc resumeTestCase) {
						require.Equal(t, 2, int(streamCreated.Load()), tc.name)
					},
				}
			},
		},
		{
			name:    "stream created when checkpoint is lost after failure",
			retries: 1,
			onRetry: func(t *testing.T, attempt int, uploader *Uploader) {
				files, err := os.ReadDir(uploader.cfg.ScanDir)
				require.NoError(t, err)
				checkpointsDeleted := 0
				for i := range files {
					fi := files[i]
					if fi.IsDir() {
						continue
					}
					if filepath.Ext(fi.Name()) == checkpointExt {
						err := os.Remove(filepath.Join(uploader.cfg.ScanDir, fi.Name()))
						require.NoError(t, err)
						t.Logf("Deleted checkpoint file: %v.", fi.Name())
						checkpointsDeleted++
					}
				}
				require.Equal(t, 1, checkpointsDeleted, "expected to delete checkpoint file")
			},
			newTest: func(streamer events.Streamer) resumeTestTuple {
				var streamCreated, terminateConnection, streamResumed atomic.Uint64
				terminateConnection.Store(1)

				callbackStreamer, err := events.NewCallbackStreamer(events.CallbackStreamerConfig{
					Inner: streamer,
					OnRecordEvent: func(ctx context.Context, sid session.ID, pe apievents.PreparedSessionEvent) error {
						event := pe.GetAuditEvent()
						if event.GetIndex() > 600 && terminateConnection.CompareAndSwap(1, 0) == true {
							t.Logf("Terminating connection at event %v", event.GetIndex())
							return trace.ConnectionProblem(nil, "connection terminated")
						}
						return nil
					},
					OnCreateAuditStream: func(ctx context.Context, sid session.ID, streamer events.Streamer) (apievents.Stream, error) {
						stream, err := streamer.CreateAuditStream(ctx, sid)
						require.NoError(t, err)
						streamCreated.Add(1)
						return stream, nil
					},
					OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer events.Streamer) (apievents.Stream, error) {
						stream, err := streamer.ResumeAuditStream(ctx, sid, uploadID)
						require.NoError(t, err)
						streamResumed.Add(1)
						return stream, nil
					},
				})
				require.NoError(t, err)
				return resumeTestTuple{
					streamer: callbackStreamer,
					verify: func(t *testing.T, tc resumeTestCase) {
						require.Equal(t, 2, int(streamCreated.Load()), tc.name)
						require.Equal(t, 0, int(streamResumed.Load()), tc.name)
					},
				}
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runResume(t, tc)
		})
	}
}

// TestUploadBackoff introduces upload failure
// and makes sure that the uploader starts backing off.
func TestUploadBackoff(t *testing.T) {
	var terminateConnectionAt atomic.Int64
	terminateConnectionAt.Store(700)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	p := newUploaderPack(ctx, t, uploaderPackConfig{
		wrapProtoStreamer: func(streamer events.Streamer) (events.Streamer, error) {
			return events.NewCallbackStreamer(events.CallbackStreamerConfig{
				Inner: streamer,
				OnRecordEvent: func(ctx context.Context, sid session.ID, pe apievents.PreparedSessionEvent) error {
					event := pe.GetAuditEvent()
					terminateAt := terminateConnectionAt.Load()
					if terminateAt > 0 && event.GetIndex() >= terminateAt {
						t.Logf("Terminating connection at event %v", event.GetIndex())
						return trace.ConnectionProblem(nil, "connection terminated at event index %v", terminateAt)
					}
					return nil
				},
			})
		},
	})

	// wait until uploader blocks on the clock before creating the stream
	p.clock.BlockUntilContext(ctx, 1)

	inEvents := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 4096})
	sid := inEvents[0].(events.SessionMetadataGetter).GetSessionID()

	stream, err := p.fileStreamer.CreateAuditStream(ctx, session.ID(sid))
	require.NoError(t, err)

	for _, event := range inEvents {
		err := stream.RecordEvent(ctx, eventstest.PrepareEvent(event))
		require.NoError(t, err)
	}
	err = stream.Complete(ctx)
	require.NoError(t, err)

	// initiate the scan by advancing clock past
	// block period
	p.clock.Advance(p.scanPeriod + time.Second)

	// initiate several scan attempts and increase the scan period
	attempts := 10
	var prev time.Time
	var diffs []time.Duration
	for i := range attempts {
		// wait for the upload event
		var event events.UploadEvent
		select {
		case event = <-p.eventsC:
			require.Error(t, event.Error)
			if prev.IsZero() {
				prev = event.Created
			} else {
				diffs = append(diffs, event.Created.Sub(prev))
				prev = event.Created
			}
		case <-ctx.Done():
			t.Fatalf("Timeout waiting for async upload %v, try `go test -v` to get more logs for details", i)
		}

		// Block until Scan has been called two times,
		// first time after doing the scan, and second
		// on receiving the event to <- eventsCh
		p.clock.BlockUntilContext(ctx, 2)
		p.clock.Advance(p.scanPeriod*time.Duration(i+2) + time.Second)
	}

	// Make sure that durations between retries are increasing
	for i, diff := range diffs {
		if i > 0 {
			require.Greater(t, diff, diffs[i-1], "Expected next retry to take longer, got %v vs %v", diffs[i-1], diff)
		}
	}

	// Fix the streamer, make sure the upload succeeds
	terminateConnectionAt.Store(0)
	p.clock.BlockUntilContext(ctx, 2)
	p.clock.Advance(time.Hour)
	select {
	case event := <-p.eventsC:
		require.NoError(t, event.Error)
	case <-ctx.Done():
		t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
	}
}

// TestUploadBadSession creates a corrupted session file
// and makes sure the uploader marks it as faulty
func TestUploadBadSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	p := newUploaderPack(ctx, t, uploaderPackConfig{})

	// wait until uploader blocks on the clock
	p.clock.BlockUntilContext(ctx, 1)

	sessionID := session.NewID()
	fileName := filepath.Join(p.scanDir, string(sessionID)+tarExt)

	err := os.WriteFile(fileName, []byte("this session is corrupted"), 0o600)
	require.NoError(t, err)

	// initiate the scan by advancing clock past
	// block period
	p.clock.Advance(p.scanPeriod + time.Second)

	// wait for the upload event, make sure
	// the error is the problem error
	var event events.UploadEvent
	select {
	case event = <-p.eventsC:
		require.Error(t, event.Error)
		require.True(t, isSessionError(event.Error))
	case <-ctx.Done():
		t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
	}

	stats, err := p.uploader.Scan(ctx)
	require.NoError(t, err)
	// Bad records have been scanned, but uploads have not started
	require.Equal(t, 1, stats.Scanned)
	require.Equal(t, 0, stats.Started)
}

// TestMinimumUpload tests that the minimum upload values for files and final uploads are respected.
func TestMinimumUpload(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	// The gzip writer ensures that upload parts exceed the minimum upload size, and imprecisely
	// constrains the maximum size to 133% of the minimum. At small values, this is especially
	// imprecise, so we give a full 200% of the minimum as overhead.
	//
	// Usually, we use 128KB for file parts and 5KB for final upload parts.
	minFileBytes := 8192
	maxFileBytes := 2 * minFileBytes
	minUploadBytes := 33768
	maxUploadBytes := 2 * minUploadBytes

	p := newUploaderPack(ctx, t, uploaderPackConfig{
		minimumFileUploadBytes: int64(minFileBytes),
		minimumUploadBytes:     int64(minUploadBytes),
	})

	// wait until uploader blocks on the clock
	p.clock.BlockUntilContext(ctx, 1)

	inEvents := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: int64(mathrand.IntN(5000) + 5000)})
	sid := inEvents[0].(events.SessionMetadataGetter).GetSessionID()
	stream, err := p.fileStreamer.CreateAuditStream(ctx, session.ID(sid))
	require.NoError(t, err)
	for _, event := range inEvents {
		err := stream.RecordEvent(ctx, eventstest.PrepareEvent(event))
		require.NoError(t, err)
	}

	// Before completing the file stream, which writes upload part files into a .tar, check that
	// each file part was within specific size expectations.
	var partFiles []fs.FileInfo
	filepath.WalkDir(p.scanDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return trace.Wrap(err, "walking directory tree")
		}

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return trace.Wrap(err, "retrieving file info")
		}

		// skip non-part files.
		if ext := filepath.Ext(info.Name()); ext == ".part" {
			partFiles = append(partFiles, info)
		}

		return nil
	})
	require.NoError(t, err)

	slices.SortFunc(partFiles, func(a fs.FileInfo, b fs.FileInfo) int {
		aPartNumberStr, _ := strings.CutSuffix(a.Name(), ".part")
		aPartNumber, err := strconv.Atoi(aPartNumberStr)
		require.NoError(t, err)
		bPartNumberStr, _ := strings.CutSuffix(b.Name(), ".part")
		bPartNumber, err := strconv.Atoi(bPartNumberStr)
		require.NoError(t, err)
		return aPartNumber - bPartNumber
	})

	for i, partFile := range partFiles {
		partSize := int(partFile.Size())
		require.GreaterOrEqual(t, partSize, minFileBytes, "expected upload part %v to be between %v and %v bytes, but was %v bytes", i, minFileBytes, maxFileBytes, partSize)
		require.LessOrEqual(t, partSize, maxFileBytes, "expected upload part %v to be between %v and %v bytes, but was %v bytes", i, minFileBytes, maxFileBytes, partSize)
	}

	// Complete the file stream and advance the clock to unblock the uploader scanner.
	err = stream.Complete(ctx)
	require.NoError(t, err)
	p.clock.Advance(p.scanPeriod + time.Second)

	var event events.UploadEvent
	select {
	case event = <-p.memEventsC:
		require.Equal(t, event.SessionID, sid)
		require.NoError(t, event.Error)
	case <-ctx.Done():
		t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
	}

	// read the upload and make sure the data is equal
	outEvents := p.readEvents(ctx, t, event.UploadID)
	require.Equal(t, inEvents, outEvents)

	uploadParts, err := p.memUploader.GetParts(event.UploadID)
	require.NoError(t, err)

	// minimumProtoUploadBytes should ensure each upload part is within specific size expectations.
	for i, part := range uploadParts {
		if i == len(uploadParts)-1 {
			// The last part is not required to meet the minimum size.
			require.LessOrEqual(t, len(part), maxUploadBytes, "expected last upload part to be smaller than %v bytes, but was %v bytes", maxUploadBytes, len(part))
		} else {
			require.GreaterOrEqual(t, len(part), minUploadBytes, "expected upload part %v to be between %v and %v bytes, but was %v bytes", i, minUploadBytes, maxUploadBytes, len(part))
			require.LessOrEqual(t, len(part), maxUploadBytes, "expected upload part %v to be between %v and %v bytes, but was %v bytes", i, minUploadBytes, maxUploadBytes, len(part))
		}
	}

	// There should be at least 1 final upload part for every 4 file upload parts.
	minFactor := minUploadBytes / minFileBytes
	require.LessOrEqual(t, len(partFiles)/minFactor, len(uploadParts), "expected there to be 1 final upload part for every 4 transient file parts, but got %v and %v respectively", len(uploadParts), len(partFiles))
}

func TestUploadEncryptedRecording(t *testing.T) {
	for _, tc := range []struct {
		name        string
		minFileSize int
		// encrypted upload files should be aggregated by the encrypted uploader to reach the target size.
		encryptedTargetSize      int
		encryptedMaxSize         int
		expectEncryptedUploadErr bool
		// uploads from the encrypted uploader should be padded further by the final uploader to
		// reach the minimum upload size. e.g. pad encrypted uploads of 4MB (max from GRPC) to reach
		// S3 minimum upload of 5MB.
		minUploadBytes int
	}{
		{
			name:                "target size larger than max encrypted upload size",
			minFileSize:         8192,
			encryptedTargetSize: 8192 * 4 * 2,
			encryptedMaxSize:    8192 * 4,
		}, {
			name:                "target size equals max encrypted upload size",
			minFileSize:         8192,
			encryptedTargetSize: 8192 * 4,
			encryptedMaxSize:    8192 * 4,
		}, {
			name:                "target size smaller than max encrypted upload size",
			minFileSize:         8192,
			encryptedTargetSize: 8192 * 4,
			encryptedMaxSize:    8192 * 4 * 2,
		}, {
			name:                "target size larger than min upload size",
			minFileSize:         8192,
			encryptedTargetSize: 8192 * 4 * 2,
			minUploadBytes:      8192 * 4,
		}, {
			name:                "target size equals min upload size",
			minFileSize:         8192,
			encryptedTargetSize: 8192 * 4,
			minUploadBytes:      8192 * 4,
		}, {
			name:                "target size smaller than min upload size",
			minFileSize:         8192,
			encryptedTargetSize: 8192 * 4,
			minUploadBytes:      8192 * 4 * 2,
		}, {
			name:                     "min file size larger than max encrypted size",
			minFileSize:              8192 * 4,
			encryptedMaxSize:         8192,
			expectEncryptedUploadErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			// First we calculate some expected size values for different steps in the event stream.
			// It is easier to calculate them here rather than in the test case's parameters.

			// the gzip writer is imprecise and may exceed the min file size. With a min of 8192,
			// an extra 8192 bytes should be plenty of overhead. If this test becomes flaky, consider
			// raising the minimum in the test cases above.
			maxFileSize := tc.minFileSize * 2

			// We expect encrypted uploads to exceed the target size, unless the maximum prevents it,
			// in which case we will be short one file part.
			expectEncryptedSizeFloor := tc.encryptedTargetSize
			if tc.encryptedMaxSize != 0 && tc.encryptedTargetSize+maxFileSize > tc.encryptedMaxSize {
				expectEncryptedSizeFloor = tc.encryptedMaxSize - maxFileSize
			}

			// We expect encrypted uploads to be below the max size if set. Otherwise, we expect it to
			// be the target size + one additional file part.
			expectEncryptedSizeCeil := tc.encryptedMaxSize
			if tc.encryptedMaxSize == 0 {
				expectEncryptedSizeCeil = tc.encryptedTargetSize + maxFileSize
			}

			// the final upload is never smaller than the encrypted size, but always above the minimum.
			expectFinalSizeFloor := max(tc.minUploadBytes, expectEncryptedSizeFloor)

			// the final upload is only larger than the minimum (+header_size) if the encrypted size is larger.
			expectFinalSizeCeil := max(expectEncryptedSizeCeil, tc.minUploadBytes+events.ProtoStreamV2PartHeaderSize)

			// Create a wrapper around the encrypted uploader to ensure the caller is yielding
			// correctly sized uploads.
			var recollectParts [][]byte
			encryptedUploadWrapper := wrapEncryptedUploaderFn(func(u events.EncryptedRecordingUploader) events.EncryptedRecordingUploader {
				return encryptedUploaderFn(func(ctx context.Context, sessionID string, parts iter.Seq2[[]byte, error]) error {
					next, stop := iter.Pull2(parts)
					defer stop()

					part, err, ok := next()
					if err != nil {
						return trace.Wrap(err)
					} else if !ok {
						return trace.BadParameter("unexpected empty upload")
					}

					for {
						recollectParts = append(recollectParts, part)

						nextPart, err, hasNext := next()
						if err != nil {
							return trace.Wrap(err)
						}

						if !hasNext {
							break
						}

						if hasNext {
							require.GreaterOrEqual(t, len(part), expectEncryptedSizeFloor, "expected encrypted upload to be between %v and %v bytes, but was %v bytes", expectEncryptedSizeFloor, expectEncryptedSizeCeil, len(part))
							require.LessOrEqual(t, len(part), expectEncryptedSizeCeil, "expected encrypted upload to be between %v and %v bytes, but was %v bytes", expectEncryptedSizeFloor, expectEncryptedSizeCeil, len(part))
						} else {
							// The last part is not expected to meet the target size.
							require.LessOrEqual(t, len(part), expectEncryptedSizeCeil, "expected last encrypted upload to be smaller than %v bytes, but was %v bytes", expectEncryptedSizeCeil, len(part))
						}

						part = nextPart
					}

					partReIter := func(yield func([]byte, error) bool) {
						for _, part := range recollectParts {
							if !yield(part, nil) {
								return
							}
						}
					}

					return u.UploadEncryptedRecording(ctx, sessionID, partReIter)
				})
			})

			p := newUploaderPack(ctx, t, uploaderPackConfig{
				minimumFileUploadBytes:             int64(tc.minFileSize),
				minimumUploadBytes:                 int64(tc.minUploadBytes),
				encrypter:                          &fakeEncryptedIO{},
				wrapEncryptedUploader:              encryptedUploadWrapper,
				encryptedRecordingUploadTargetSize: tc.encryptedTargetSize,
				encryptedRecordingUploadMaxSize:    tc.encryptedMaxSize,
			})

			// wait until uploader blocks on the clock
			err := p.clock.BlockUntilContext(ctx, 1)
			require.NoError(t, err)

			// Here we ensure at least 5 final upload parts so that we amply test the test case values, + some variance.
			eventsCount := expectFinalSizeCeil*5/64 + mathrand.IntN(1000)
			inEvents := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: int64(eventsCount)})
			sid := inEvents[0].(events.SessionMetadataGetter).GetSessionID()
			p.emitEvents(ctx, t, inEvents)

			// initiate the scan by advancing clock past
			// block period
			p.clock.Advance(p.scanPeriod + time.Second)

			var event events.UploadEvent
			select {
			case event = <-p.memEventsC:
				if tc.expectEncryptedUploadErr {
					t.Fatalf("Unexpected upload event")
				}
				require.Equal(t, event.SessionID, sid)
				require.NoError(t, event.Error)
			case event = <-p.eventsC:
				if !tc.expectEncryptedUploadErr {
					t.Fatalf("Unexpected upload event")
				}
				require.Error(t, event.Error)
				require.True(t, isSessionError(event.Error))
				return
			case <-ctx.Done():
				t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
			}

			// read the upload and make sure the data is equal
			outEvents := p.readEvents(ctx, t, event.UploadID)
			require.Equal(t, inEvents, outEvents)

			uploadParts, err := p.memUploader.GetParts(event.UploadID)
			require.NoError(t, err)

			// final uploads should be above the minimum upload size.
			for i, part := range uploadParts {
				if i == len(uploadParts)-1 {
					// The last part is not required to meet the minimum size, so it shouldn't exceed the original encrypted recording size.
					require.LessOrEqual(t, len(part), expectEncryptedSizeCeil, "expected last upload to be smaller than %v bytes, but was %v bytes", expectEncryptedSizeCeil, len(part))
				} else {
					require.GreaterOrEqual(t, len(part), expectFinalSizeFloor, "expected upload to be between %v and %v bytes, but was %v bytes", expectFinalSizeFloor, expectFinalSizeCeil, len(part))
					require.LessOrEqual(t, len(part), expectFinalSizeCeil, "expected upload to be between %v and %v bytes, but was %v bytes", expectFinalSizeFloor, expectFinalSizeCeil, len(part))
				}
			}

			// There should be one final upload for each upload part from the encrypted uploader.
			require.Len(t, recollectParts, len(uploadParts), "expected there to be an equal amount of final upload parts and transient upload parts, but got %v and %v respectively", len(uploadParts), len(recollectParts))
		})
	}
}

type uploaderPackConfig struct {
	minimumFileUploadBytes             int64
	minimumUploadBytes                 int64
	wrapProtoStreamer                  wrapStreamerFn
	encrypter                          events.EncryptionWrapper
	wrapEncryptedUploader              wrapEncryptedUploaderFn
	encryptedRecordingUploadTargetSize int
	encryptedRecordingUploadMaxSize    int
}

// uploaderPack reduces boilerplate required
// to create a test
type uploaderPack struct {
	scanPeriod       time.Duration
	initialScanDelay time.Duration
	clock            *clockwork.FakeClock
	// fileStreamer streams events to upload parts on disk.
	scanDir      string
	fileStreamer events.Streamer
	// uploader scans upload parts from disk and streams them through the protoStreamer.
	eventsC  chan events.UploadEvent
	uploader *Uploader
	// protoStreamer streams events to upload parts in-memory (represents final audit log storage).
	protoStreamer events.Streamer
	memEventsC    chan events.UploadEvent
	memUploader   *eventstest.MemoryUploader
}

func newUploaderPack(ctx context.Context, t *testing.T, cfg uploaderPackConfig) uploaderPack {
	scanDir := t.TempDir()
	corruptedDir := t.TempDir()

	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	pack := uploaderPack{
		clock:            clockwork.NewFakeClock(),
		eventsC:          make(chan events.UploadEvent, 100),
		memEventsC:       make(chan events.UploadEvent, 100),
		scanDir:          scanDir,
		scanPeriod:       10 * time.Second,
		initialScanDelay: 10 * time.Millisecond,
	}

	pack.memUploader = eventstest.NewMemoryUploader(eventstest.MemoryUploaderConfig{
		EventsC:            pack.memEventsC,
		MinimumUploadBytes: int(cfg.minimumUploadBytes),
	})

	var err error
	pack.fileStreamer, err = NewStreamer(StreamerConfig{
		Dir:            scanDir,
		MinUploadBytes: cfg.minimumFileUploadBytes,
		Encrypter:      cfg.encrypter,
	})
	require.NoError(t, err)

	pack.protoStreamer, err = events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader:       pack.memUploader,
		MinUploadBytes: cfg.minimumUploadBytes,
		// Skip encrypter in proto streamer, encryption skips the proto stream flow.
	})
	require.NoError(t, err)

	if cfg.wrapProtoStreamer != nil {
		pack.protoStreamer, err = cfg.wrapProtoStreamer(pack.protoStreamer)
		require.NoError(t, err)
	}

	uploaderCfg := UploaderConfig{
		ScanDir:                            pack.scanDir,
		CorruptedDir:                       corruptedDir,
		InitialScanDelay:                   pack.initialScanDelay,
		ScanPeriod:                         pack.scanPeriod,
		Streamer:                           pack.protoStreamer,
		Clock:                              pack.clock,
		EventsC:                            pack.eventsC,
		EncryptedRecordingUploader:         pack.memUploader,
		EncryptedRecordingUploadTargetSize: cfg.encryptedRecordingUploadTargetSize,
		EncryptedRecordingUploadMaxSize:    cfg.encryptedRecordingUploadMaxSize,
	}
	if cfg.wrapEncryptedUploader != nil {
		uploaderCfg.EncryptedRecordingUploader = cfg.wrapEncryptedUploader(pack.memUploader)
	}

	pack.uploader, err = NewUploader(uploaderCfg)
	require.NoError(t, err)

	go pack.uploader.Serve(ctx)

	return pack
}

func (p *uploaderPack) emitEvents(ctx context.Context, t *testing.T, inEvents []apievents.AuditEvent) {
	emitStream(ctx, t, p.fileStreamer, inEvents)
}

func (p *uploaderPack) readEvents(ctx context.Context, t *testing.T, uploadID string) []apievents.AuditEvent {
	return readStream(ctx, t, uploadID, p.memUploader)
}

type wrapStreamerFn func(streamer events.Streamer) (events.Streamer, error)

type wrapEncryptedUploaderFn func(u events.EncryptedRecordingUploader) events.EncryptedRecordingUploader

// runResume runs resume scenario based on the test case specification
func runResume(t *testing.T, testCase resumeTestCase) {
	t.Logf("Running test %q.", testCase.name)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	clock := clockwork.NewFakeClock()
	eventsC := make(chan events.UploadEvent, 100)
	memUploader := eventstest.NewMemoryUploader(eventstest.MemoryUploaderConfig{
		EventsC: eventsC,
	})
	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader:       memUploader,
		MinUploadBytes: 1024,
	})
	require.NoError(t, err)

	test := testCase.newTest(streamer)

	scanDir := t.TempDir()
	corruptedDir := t.TempDir()

	scanPeriod := 10 * time.Second
	uploader, err := NewUploader(UploaderConfig{
		EventsC:          eventsC,
		ScanDir:          scanDir,
		CorruptedDir:     corruptedDir,
		InitialScanDelay: 10 * time.Millisecond,
		ScanPeriod:       scanPeriod,
		Streamer:         test.streamer,
		Clock:            clock,
	})
	require.NoError(t, err)
	go uploader.Serve(ctx)
	// wait until uploader blocks on the clock
	clock.BlockUntilContext(ctx, 1)

	defer uploader.Close()

	fileStreamer, err := NewStreamer(StreamerConfig{
		Dir: scanDir,
	})
	require.NoError(t, err)

	inEvents := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 1024})
	sid := inEvents[0].(events.SessionMetadataGetter).GetSessionID()

	emitStream(ctx, t, fileStreamer, inEvents)

	// initiate the scan by advancing clock past
	// block period
	clock.Advance(scanPeriod + time.Second)

	// wait for the upload failure
	var event events.UploadEvent
	select {
	case event = <-eventsC:
		require.Equal(t, event.SessionID, sid)
		require.IsType(t, trace.ConnectionProblem(nil, "connection problem"), event.Error)
	case <-ctx.Done():
		t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
	}

	for i := range testCase.retries {
		if testCase.onRetry != nil {
			testCase.onRetry(t, i, uploader)
		}
		// Block until Scan has been called two times,
		// first time after doing the scan, and second
		// on receiving the event to <- eventsCh
		clock.BlockUntilContext(ctx, 2)
		clock.Advance(scanPeriod*time.Duration(i+2) + time.Second)

		// wait for upload success
		select {
		case event = <-eventsC:
			require.Equal(t, event.SessionID, sid)
			if i == testCase.retries-1 {
				require.NoError(t, event.Error)
			} else {
				require.IsType(t, trace.ConnectionProblem(nil, "connection problem"), event.Error)
			}
		case <-ctx.Done():
			t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
		}
	}

	// read the upload and make sure the data is equal
	outEvents := readStream(ctx, t, event.UploadID, memUploader)

	require.Equal(t, inEvents, outEvents)

	// perform additional checks as defined by test case
	test.verify(t, testCase)
}

// emitStream creates and sends the session stream
func emitStream(ctx context.Context, t *testing.T, streamer events.Streamer, inEvents []apievents.AuditEvent) {
	t.Helper()

	sid := inEvents[0].(events.SessionMetadataGetter).GetSessionID()

	stream, err := streamer.CreateAuditStream(ctx, session.ID(sid))
	require.NoError(t, err)
	for _, event := range inEvents {
		err := stream.RecordEvent(ctx, eventstest.PrepareEvent(event))
		require.NoError(t, err)
	}
	err = stream.Complete(ctx)
	require.NoError(t, err)
}

// readStream reads and decodes the audit stream from uploadID
func readStream(ctx context.Context, t *testing.T, uploadID string, uploader *eventstest.MemoryUploader) []apievents.AuditEvent {
	t.Helper()

	parts, err := uploader.GetParts(uploadID)
	require.NoError(t, err)

	var outEvents []apievents.AuditEvent
	var reader *events.ProtoReader
	for i, part := range parts {
		if i == 0 {
			reader = events.NewProtoReader(bytes.NewReader(part), &fakeEncryptedIO{})
		} else {
			err := reader.Reset(bytes.NewReader(part))
			require.NoError(t, err)
		}
		out, err := reader.ReadAll(ctx)
		require.NoError(t, err, "part crash %#v", part)

		outEvents = append(outEvents, out...)
	}
	return outEvents
}

// encryptedIO is really just a reversible transform, so we fake encryption by encoding/decoding as hex
type fakeEncryptedIO struct {
	err error
}

type fakeEncrypter struct {
	inner  io.WriteCloser
	writer io.Writer
}

func (f *fakeEncrypter) Write(out []byte) (int, error) {
	return f.writer.Write(out)
}

func (f *fakeEncrypter) Close() error {
	return f.inner.Close()
}

func (f *fakeEncryptedIO) WithEncryption(ctx context.Context, writer io.WriteCloser) (io.WriteCloser, error) {
	hexWriter := hex.NewEncoder(writer)
	encrypter := &fakeEncrypter{
		inner:  writer,
		writer: hexWriter,
	}

	return encrypter, f.err
}

func (f *fakeEncryptedIO) WithDecryption(ctx context.Context, reader io.Reader) (io.Reader, error) {
	return hex.NewDecoder(reader), f.err
}

type encryptedUploaderFn func(ctx context.Context, sessionID string, parts iter.Seq2[[]byte, error]) error

func (e encryptedUploaderFn) UploadEncryptedRecording(ctx context.Context, sessionID string, parts iter.Seq2[[]byte, error]) error {
	return e(ctx, sessionID, parts)
}
