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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

// UploaderConfig sets up configuration for uploader service
type UploaderConfig struct {
	// ScanDir is data directory with the uploads
	ScanDir string
	// CorruptedDir is the directory to store corrupted uploads in.
	CorruptedDir string
	// Clock is the clock replacement
	Clock clockwork.Clock
	// InitialScanDelay is how long to wait before performing the initial scan.
	InitialScanDelay time.Duration
	// ScanPeriod is a uploader dir scan period
	ScanPeriod time.Duration
	// ConcurrentUploads sets up how many parallel uploads to schedule
	ConcurrentUploads int
	// Streamer is upstream streamer to upload events to
	Streamer events.Streamer
	// EventsC is an event channel used to signal events
	// used in tests
	EventsC chan events.UploadEvent
	// Component is used for logging purposes
	Component string
	// EncryptedRecordingUploader uploads encrypted session recordings
	EncryptedRecordingUploader events.EncryptedRecordingUploader
}

// CheckAndSetDefaults checks and sets default values of UploaderConfig
func (cfg *UploaderConfig) CheckAndSetDefaults() error {
	if cfg.Streamer == nil {
		return trace.BadParameter("missing parameter Streamer")
	}
	if cfg.ScanDir == "" {
		return trace.BadParameter("missing parameter ScanDir")
	}
	if cfg.CorruptedDir == "" {
		return trace.BadParameter("missing parameter CorruptedDir")
	}
	if cfg.ConcurrentUploads <= 0 {
		cfg.ConcurrentUploads = defaults.UploaderConcurrentUploads
	}
	if cfg.ScanPeriod <= 0 {
		cfg.ScanPeriod = defaults.UploaderScanPeriod
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.Component == "" {
		cfg.Component = teleport.ComponentUpload
	}
	return nil
}

// NewUploader creates new disk based session logger
func NewUploader(cfg UploaderConfig) (*Uploader, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := os.MkdirAll(cfg.ScanDir, teleport.SharedDirMode); err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	if err := os.MkdirAll(cfg.CorruptedDir, teleport.SharedDirMode); err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	uploader := &Uploader{
		cfg:           cfg,
		log:           slog.With(teleport.ComponentKey, cfg.Component),
		closeC:        make(chan struct{}),
		semaphore:     make(chan struct{}, cfg.ConcurrentUploads),
		eventsCh:      make(chan events.UploadEvent, cfg.ConcurrentUploads),
		eventPreparer: &events.NoOpPreparer{},
	}
	return uploader, nil
}

// Uploader periodically scans session records in a folder.
//
// Once it finds the sessions it opens parallel upload streams
// to the streaming server.
//
// It keeps checkpoints of the upload state and resumes
// the upload that have been aborted.
//
// It marks corrupted session files to skip their processing.
type Uploader struct {
	semaphore chan struct{}

	cfg UploaderConfig
	log *slog.Logger

	eventsCh  chan events.UploadEvent
	closeC    chan struct{}
	wg        sync.WaitGroup
	mu        sync.Mutex
	isClosing bool

	eventPreparer *events.NoOpPreparer
}

func (u *Uploader) Close() {
	// TODO(tigrato): prevent close to be called before Serve starts.
	u.mu.Lock()
	u.isClosing = true
	u.mu.Unlock()

	close(u.closeC)
	// wait for all uploads to finish
	u.wg.Wait()
}

func (u *Uploader) writeSessionError(sessionID session.ID, err error) error {
	if sessionID == "" {
		return trace.BadParameter("missing session ID")
	}
	path := u.sessionErrorFilePath(sessionID)
	return trace.ConvertSystemError(os.WriteFile(path, []byte(err.Error()), 0o600))
}

func (u *Uploader) checkSessionError(sessionID session.ID) (bool, error) {
	if sessionID == "" {
		return false, trace.BadParameter("missing session ID")
	}
	_, err := os.Stat(u.sessionErrorFilePath(sessionID))
	if err != nil {
		err = trace.ConvertSystemError(err)
		if trace.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Serve runs the uploader until stopped
func (u *Uploader) Serve(ctx context.Context) error {
	// Check if close operation is already in progress.
	// We need to do this because Serve is spawned in a goroutine
	// and Close can be called before Serve starts which ends up in a data
	// race because Close is waiting for wg to be 0 and Serve is adding to wg.
	// To avoid this, we check if Close is already in progress and return
	// immediately. If Close is not in progress, we add to wg under the mutex
	// lock to ensure that Close can't reach wg.Wait() before Serve adds to wg.
	u.mu.Lock()
	if u.isClosing {
		u.mu.Unlock()
		return nil
	}
	u.wg.Add(1)
	u.mu.Unlock()
	defer u.wg.Done()

	u.log.InfoContext(ctx, "uploader server ready", "scan_dir", u.cfg.ScanDir, "scan_period", u.cfg.ScanPeriod.String())
	backoff, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  u.cfg.InitialScanDelay,
		Step:   u.cfg.ScanPeriod,
		Max:    u.cfg.ScanPeriod * 100,
		Clock:  u.cfg.Clock,
		Jitter: retryutils.SeventhJitter,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	for {
		select {
		case <-u.closeC:
			return nil
		case <-ctx.Done():
			return nil
		case event := <-u.eventsCh:
			// Successful and failed upload events are used to speed up and
			// slow down the scans and uploads.
			switch {
			case event.Error == nil:
				backoff.ResetToDelay()
			case isSessionError(event.Error):
				u.log.WarnContext(ctx, "Failed to read session recording, will skip future uploads.", "session_id", event.SessionID)
				if err := u.writeSessionError(session.ID(event.SessionID), event.Error); err != nil {
					u.log.WarnContext(ctx, "Failed to write session", "error", err, "session_id", event.SessionID)
				}
			default:
				backoff.Inc()
				u.log.WarnContext(ctx, "Increasing session upload backoff due to error, applying backoff before retrying", "backoff", backoff.Duration())
			}
			// forward the event to channel that used in tests
			if u.cfg.EventsC != nil {
				select {
				case u.cfg.EventsC <- event:
				default:
					u.log.WarnContext(ctx, "Skip send event on a blocked channel.")
				}
			}
		// Tick at scan period but slow down (and speeds up) on errors.
		case <-backoff.After():
			if _, err := u.Scan(ctx); err != nil {
				if !errors.Is(trace.Unwrap(err), errContext) {
					backoff.Inc()
					u.log.WarnContext(ctx, "Uploader scan failed, applying backoff before retrying", "backoff", backoff.Duration(), "error", err)
				}
			}
		}
	}
}

// ScanStats provides scan statistics,
// used in tests
type ScanStats struct {
	// Scanned is how many uploads have been scanned
	Scanned int
	// Started is how many uploads have been started
	Started int
	// Corrupted is how many corrupted uploads have been
	// moved out of the scan dir.
	Corrupted int
}

// Scan scans the streaming directory and uploads recordings
func (u *Uploader) Scan(ctx context.Context) (*ScanStats, error) {
	files, err := os.ReadDir(u.cfg.ScanDir)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	var stats ScanStats
	for i := range files {
		fi := files[i]
		if fi.IsDir() {
			continue
		}
		ext := filepath.Ext(fi.Name())
		if ext == checkpointExt || ext == errorExt {
			continue
		}
		stats.Scanned++
		if err := u.startUpload(ctx, fi.Name()); err != nil {
			if errors.Is(err, utils.ErrUnsuccessfulLockTry) {
				u.log.DebugContext(ctx, "Scan is skipping recording that is locked by another process.", "recording", fi.Name())
				continue
			}
			if trace.IsNotFound(err) {
				u.log.DebugContext(ctx, "Recording was uploaded by another process.", "recording", fi.Name())
				continue
			}
			if isSessionError(err) || trace.IsBadParameter(err) {
				u.log.WarnContext(ctx, "Skipped session recording.", "recording", fi.Name(), "error", err)
				stats.Corrupted++
				continue
			}
			return nil, trace.Wrap(err)
		}
		stats.Started++
	}
	if stats.Scanned > 0 {
		u.log.DebugContext(ctx, "Session recording scan completed ", "scanned", stats.Scanned, "started", stats.Started, "corrupted", stats.Corrupted, "upload_dir", u.cfg.ScanDir)
	}
	return &stats, nil
}

// checkpointFilePath returns a path to checkpoint file for a session
func (u *Uploader) checkpointFilePath(sid session.ID) string {
	return filepath.Join(u.cfg.ScanDir, sid.String()+checkpointExt)
}

// sessionErrorFilePath returns a path to checkpoint file for a session
func (u *Uploader) sessionErrorFilePath(sid session.ID) string {
	return filepath.Join(u.cfg.ScanDir, sid.String()+errorExt)
}

type upload struct {
	sessionID      session.ID
	reader         *events.ProtoReader
	file           *os.File
	fileUnlockFn   func() error
	checkpointFile *os.File
}

// readStatus reads stream status
func (u *upload) readStatus() (*apievents.StreamStatus, error) {
	data, err := io.ReadAll(u.checkpointFile)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	if len(data) == 0 {
		return nil, trace.NotFound("no status found")
	}
	var status apievents.StreamStatus
	err = utils.FastUnmarshal(data, &status)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &status, nil
}

// writeStatus writes stream status
func (u *upload) writeStatus(status apievents.StreamStatus) error {
	data, err := utils.FastMarshal(status)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = u.checkpointFile.Seek(0, 0)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	n, err := u.checkpointFile.Write(data)
	if err != nil {
		return trace.Wrap(err)
	}
	if n < len(data) {
		return trace.ConvertSystemError(io.ErrShortWrite)
	}
	return nil
}

// releaseFile releases file and associated resources
// in a correct order
func (u *upload) Close() error {
	return trace.NewAggregate(
		u.reader.Close(),
		u.fileUnlockFn(),
		u.file.Close(),
		utils.NilCloser(u.checkpointFile).Close(),
	)
}

func (u *upload) removeFiles() error {
	var errs []error
	if u.file != nil {
		errs = append(errs,
			trace.ConvertSystemError(os.Remove(u.file.Name())))
	}
	if u.checkpointFile != nil {
		errs = append(errs,
			trace.ConvertSystemError(os.Remove(u.checkpointFile.Name())))
	}
	return trace.NewAggregate(errs...)
}

var errNotEncrypted = errors.New("recording is not encrypted")

func (u *Uploader) uploadEncryptedRecording(ctx context.Context, sessionID string, in io.ReadCloser) error {
	partIter := func(yield func([]byte, error) bool) {
		buf := bytes.NewBuffer(nil)
		for {
			buf.Reset()
			header, err := events.ParsePartHeader(in)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				yield(nil, trace.Wrap(err))
				return
			}

			if header.Flags&events.ProtoStreamFlagEncrypted == 0 {
				yield(nil, trace.Wrap(errNotEncrypted))
				return
			}

			if _, err = buf.Write(header.Bytes()); err != nil {
				yield(nil, trace.Wrap(err))
				return
			}

			totalPartSize := int64(header.PartSize + header.PaddingSize)
			reader := io.LimitReader(in, totalPartSize)
			copied, err := io.Copy(buf, reader)
			if err != nil && err != io.EOF {
				yield(nil, trace.Wrap(err))
				return
			}

			if copied != totalPartSize {
				yield(nil, trace.Errorf("copied %d bytes of recording part instead of expected %d", copied, totalPartSize))
				return
			}

			if !yield(buf.Bytes(), nil) {
				return
			}
		}
	}

	if err := u.cfg.EncryptedRecordingUploader.UploadEncryptedRecording(ctx, sessionID, partIter); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (u *Uploader) startUpload(ctx context.Context, fileName string) (err error) {
	sessionID, err := sessionIDFromPath(fileName)
	if err != nil {
		return trace.Wrap(err)
	}

	log := u.log.With(fieldSessionID, sessionID)

	sessionFilePath := filepath.Join(u.cfg.ScanDir, fileName)
	// Corrupted session records can clog the uploader
	// that will indefinitely try to upload them.
	isSessionError, err := u.checkSessionError(sessionID)
	if err != nil {
		return trace.Wrap(err)
	}
	if isSessionError {
		errorFilePath := u.sessionErrorFilePath(sessionID)
		// move the corrupted recording and the error marker to a separate directory
		// to prevent the uploader from spinning on the same corrupted upload
		var moveErrs []error
		if err := os.Rename(sessionFilePath, filepath.Join(u.cfg.CorruptedDir, filepath.Base(sessionFilePath))); err != nil {
			moveErrs = append(moveErrs, trace.Wrap(err, "moving %v to %v", sessionFilePath, u.cfg.CorruptedDir))
		}
		if err := os.Rename(errorFilePath, filepath.Join(u.cfg.CorruptedDir, filepath.Base(errorFilePath))); err != nil {
			moveErrs = append(moveErrs, trace.Wrap(err, "moving %v to %v", errorFilePath, u.cfg.CorruptedDir))
		}
		if len(moveErrs) > 0 {
			log.ErrorContext(ctx, "Failed to move corrupted recording", "error", trace.NewAggregate(moveErrs...))
		}

		return sessionError{
			err: trace.BadParameter(
				"session recording %v; check the %v directory for artifacts",
				sessionID, u.cfg.CorruptedDir),
		}
	}

	start := time.Now()
	if err := u.takeSemaphore(ctx); err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err != nil {
			_ = u.releaseSemaphore(ctx)
		}
	}()

	if time.Since(start) > 500*time.Millisecond {
		log.DebugContext(ctx, "Semaphore acquired in for upload", "time_to_acquire", time.Since(start), "upload", fileName)
	}

	// Apparently, exclusive lock can be obtained only in RDWR mode on NFS
	sessionFile, err := os.OpenFile(sessionFilePath, os.O_RDWR, 0)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	unlock, err := utils.FSTryWriteLock(sessionFilePath)
	if err != nil {
		if e := sessionFile.Close(); e != nil {
			log.WarnContext(ctx, "Failed to close", "error", err, "upload", fileName)
		}
		return trace.Wrap(err, "uploader could not acquire file lock for %q", sessionFilePath)
	}

	if u.cfg.EncryptedRecordingUploader != nil {
		if err := u.uploadEncryptedRecording(ctx, sessionID.String(), sessionFile); err != nil {
			if !errors.Is(err, errNotEncrypted) {
				return trace.Wrap(err)
			}

			// if the file isn't encrypted, seek to the beginning and proceed
			// with per-event upload
			if _, err := sessionFile.Seek(0, io.SeekStart); err != nil {
				return trace.Wrap(err)
			}
		} else {
			return trace.ConvertSystemError(os.Remove(sessionFile.Name()))
		}
	}

	protoReader := events.NewProtoReader(sessionFile, nil)

	upload := &upload{
		sessionID:    sessionID,
		reader:       protoReader,
		file:         sessionFile,
		fileUnlockFn: unlock,
	}
	upload.checkpointFile, err = os.OpenFile(u.checkpointFilePath(sessionID), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		if err := upload.Close(); err != nil {
			log.WarnContext(ctx, "Failed to close upload.", "error", err)
		}
		return trace.ConvertSystemError(err)
	}

	u.wg.Add(1)
	go func() {
		defer u.wg.Done()
		if err := u.upload(ctx, upload); err != nil {
			log.WarnContext(ctx, "Upload failed.", "error", err)
			u.emitEvent(events.UploadEvent{
				SessionID: string(upload.sessionID),
				Error:     err,
				Created:   u.cfg.Clock.Now().UTC(),
			})
			return
		}
		log.DebugContext(ctx, "Session upload completed.", "duration", time.Since(start))
		u.emitEvent(events.UploadEvent{
			SessionID: string(upload.sessionID),
			Created:   u.cfg.Clock.Now().UTC(),
		})
	}()
	return nil
}

func (u *Uploader) upload(ctx context.Context, up *upload) error {
	log := u.log.With(fieldSessionID, up.sessionID)

	defer u.releaseSemaphore(ctx)
	defer func() {
		if err := up.Close(); err != nil {
			log.WarnContext(ctx, "Failed to close upload.", "error", err)
		}
	}()

	var stream apievents.Stream
	status, err := up.readStatus()
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		stream, err = u.cfg.Streamer.CreateAuditStream(ctx, up.sessionID)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		stream, err = u.cfg.Streamer.ResumeAuditStream(ctx, up.sessionID, status.UploadID)
		if err != nil {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
			log.WarnContext(ctx, "Upload not found, starting a new upload from scratch.", "error", err, "upload", status.UploadID)
			status = nil
			stream, err = u.cfg.Streamer.CreateAuditStream(ctx, up.sessionID)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}

	// explicitly pass in the context so that the deferred
	// func doesn't observe future changes to the ctx var
	defer func(ctx context.Context) {
		if err := stream.Close(ctx); err != nil {
			if !errors.Is(trace.Unwrap(err), io.EOF) {
				log.DebugContext(ctx, "Failed to close stream.", "error", err)
			}
		}
	}(ctx)

	// The call to CreateAuditStream is async. To learn
	// if it was successful get the first status update
	// sent by the server after create.
	select {
	case <-u.closeC:
		return trace.Errorf("operation has been canceled, uploader is closed")
	case <-stream.Done():
		if errStream, ok := stream.(interface{ Error() error }); ok {
			if err := errStream.Error(); err != nil {
				return trace.ConnectionProblem(err, "%s", err.Error())
			}
		}

		return trace.ConnectionProblem(nil, "upload stream terminated unexpectedly")
	case status := <-stream.Status():
		if err := up.writeStatus(status); err != nil {
			// all other stream status writes are optimistic, but we want to make sure the initial
			// status is written to disk so that we don't create orphaned multipart uploads.
			return trace.Errorf("failed to write initial stream status: %v", err)
		}
	case <-time.After(events.NetworkRetryDuration):
		return trace.ConnectionProblem(nil, "timeout waiting for stream status update")
	case <-ctx.Done():
		return trace.ConnectionProblem(ctx.Err(), "operation has been canceled")

	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	u.wg.Add(1)
	go func() {
		defer u.wg.Done()
		u.monitorStreamStatus(ctx, up, stream, cancel)
	}()

	for {
		event, err := up.reader.Read(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return sessionError{err: trace.Wrap(err)}
		}
		// skip events that have been already submitted
		if status != nil && event.GetIndex() <= status.LastEventIndex {
			continue
		}
		// ProtoStream will only write PreparedSessionEvents, so
		// this event doesn't need to be prepared again. Convert it
		// with a NoOpPreparer.
		preparedEvent, _ := u.eventPreparer.PrepareSessionEvent(event)
		if err := stream.RecordEvent(ctx, preparedEvent); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := stream.Complete(ctx); err != nil {
		log.ErrorContext(ctx, "Failed to complete upload.", "error", err)
		return trace.Wrap(err)
	}

	// make sure that checkpoint writer goroutine finishes
	// before the files are closed to avoid async writes
	// the timeout is a defensive measure to avoid blocking
	// indefinitely in case of unforeseen error (e.g. write taking too long)
	wctx, wcancel := context.WithTimeout(ctx, apidefaults.DefaultIOTimeout)
	defer wcancel()

	<-wctx.Done()
	if errors.Is(wctx.Err(), context.DeadlineExceeded) {
		log.WarnContext(ctx, "Checkpoint function failed to complete the write due to timeout. Possible slow disk write.", "error", wctx.Err())
	}

	// In linux it is possible to remove a file while holding a file descriptor
	if err := up.removeFiles(); err != nil {
		if !trace.IsNotFound(err) {
			log.WarnContext(ctx, "Failed to remove session files.", "error", err)
		}
	}
	return nil
}

// monitorStreamStatus monitors stream's status
// and checkpoints the stream
func (u *Uploader) monitorStreamStatus(ctx context.Context, up *upload, stream apievents.Stream, cancel context.CancelFunc) {
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stream.Done():
			return
		case status := <-stream.Status():
			if err := up.writeStatus(status); err != nil {
				u.log.DebugContext(ctx, "Got stream status.", "status", status, "error", err)
			}
		}
	}
}

var errContext = fmt.Errorf("context has closed")

func (u *Uploader) takeSemaphore(ctx context.Context) error {
	select {
	case u.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return errContext
	}
}

func (u *Uploader) releaseSemaphore(ctx context.Context) error {
	select {
	case <-u.semaphore:
		return nil
	case <-ctx.Done():
		return errContext
	}
}

func (u *Uploader) emitEvent(e events.UploadEvent) {
	// This channel is used by scanner to slow down/speed up.
	select {
	case u.eventsCh <- e:
	default:
		// It's OK to drop the event if the Scan is overloaded.
		// These events are used in tests and to speed up and slow down
		// scans, so lost events will have little impact on the logic.
	}
}

func isSessionError(err error) bool {
	var sessionError sessionError
	return errors.As(trace.Unwrap(err), &sessionError)
}

// sessionError highlights problems with session
// playback, corrupted files or incompatible disk format
type sessionError struct {
	err error
}

func (s sessionError) Error() string {
	return fmt.Sprintf(
		"session file could be corrupted or is using unsupported format: %v", s.err.Error())
}

// Field names used for logging.
const (
	fieldSessionID = "session"
)
