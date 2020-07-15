/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package filesessions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

// UploaderConfig sets up configuration for uploader service
type UploaderConfig struct {
	// ScanDir is data directory with the uploads
	ScanDir string
	// Clock is the clock replacement
	Clock clockwork.Clock
	// Context is an optional context
	Context context.Context
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
}

// CheckAndSetDefaults checks and sets default values of UploaderConfig
func (cfg *UploaderConfig) CheckAndSetDefaults() error {
	if cfg.Streamer == nil {
		return trace.BadParameter("missing parameter Streamer")
	}
	if cfg.ScanDir == "" {
		return trace.BadParameter("missing parameter ScanDir")
	}
	if cfg.ConcurrentUploads <= 0 {
		cfg.ConcurrentUploads = defaults.UploaderConcurrentUploads
	}
	if cfg.ScanPeriod <= 0 {
		cfg.ScanPeriod = defaults.UploaderScanPeriod
	}
	if cfg.Context == nil {
		cfg.Context = context.Background()
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
	// completer scans for uploads that have been initiated, but not completed
	// by the client (aborted or crashed) and completed them
	handler, err := NewHandler(Config{
		Directory: cfg.ScanDir,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	uploadCompleter, err := events.NewUploadCompleter(events.UploadCompleterConfig{
		Uploader:  handler,
		Unstarted: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(cfg.Context)
	uploader := &Uploader{
		uploadCompleter: uploadCompleter,
		cfg:             cfg,
		log: log.WithFields(log.Fields{
			trace.Component: cfg.Component,
		}),
		cancel:    cancel,
		ctx:       ctx,
		semaphore: make(chan struct{}, cfg.ConcurrentUploads),
	}
	return uploader, nil
}

// Uploader implements a disk based session logger. The important
// property of the disk based logger is that it never fails and can be used as
// a fallback implementation behind more sophisticated loggers.
type Uploader struct {
	semaphore chan struct{}

	cfg             UploaderConfig
	log             *log.Entry
	uploadCompleter *events.UploadCompleter

	cancel context.CancelFunc
	ctx    context.Context
}

// Serve runs the uploader until stopped
func (u *Uploader) Serve() error {
	t := u.cfg.Clock.NewTicker(u.cfg.ScanPeriod)
	defer t.Stop()
	for {
		select {
		case <-u.ctx.Done():
			u.log.Debugf("Uploader is exiting.")
			return nil
		case <-t.Chan():
			if err := u.uploadCompleter.CheckUploads(u.ctx); err != nil {
				if trace.Unwrap(err) != errContext {
					u.log.WithError(err).Warningf("Completer scan failed.")
				}
			}
			if err := u.Scan(); err != nil {
				if trace.Unwrap(err) != errContext {
					u.log.WithError(err).Warningf("Uploader scan failed.")
				}
			}
		}
	}
}

// Scan scans the streaming directory and uploads recordings
func (u *Uploader) Scan() error {
	files, err := ioutil.ReadDir(u.cfg.ScanDir)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	u.log.Debugf("Found %v files in dir %v.", len(files), u.cfg.ScanDir)
	for i := range files {
		fi := files[i]
		if fi.IsDir() {
			continue
		}
		if filepath.Ext(fi.Name()) == checkpointExt {
			continue
		}
		if err := u.startUpload(fi.Name()); err != nil {
			if trace.IsCompareFailed(err) {
				u.log.Debugf("Uploader detected locked file %v, another process is processing it.", fi.Name())
				continue
			}
			return trace.Wrap(err)
		}
	}
	return nil
}

// checkpointFilePath  returns a path to checkpoint file for a session
func (u *Uploader) checkpointFilePath(sid session.ID) string {
	return filepath.Join(u.cfg.ScanDir, sid.String()+checkpointExt)
}

// Close closes all operations
func (u *Uploader) Close() error {
	u.cancel()
	return u.uploadCompleter.Close()
}

type upload struct {
	sessionID      session.ID
	reader         *events.ProtoReader
	file           *os.File
	checkpointFile *os.File
}

// readStatus reads stream status
func (u *upload) readStatus() (*events.StreamStatus, error) {
	data, err := ioutil.ReadAll(u.checkpointFile)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	if len(data) == 0 {
		return nil, trace.NotFound("no status found")
	}
	var status events.StreamStatus
	err = utils.FastUnmarshal(data, &status)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &status, nil
}

// writeStatus writes stream status
func (u *upload) writeStatus(status events.StreamStatus) error {
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
		utils.FSUnlock(u.file),
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

func (u *Uploader) startUpload(fileName string) error {
	sessionID, err := sessionIDFromPath(fileName)
	if err != nil {
		return trace.Wrap(err)
	}
	// Apparently, exclusive lock can be obtained only in RDWR mode on NFS
	sessionFilePath := filepath.Join(u.cfg.ScanDir, fileName)
	sessionFile, err := os.OpenFile(sessionFilePath, os.O_RDWR, 0)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	if err := utils.FSTryWriteLock(sessionFile); err != nil {
		if e := sessionFile.Close(); e != nil {
			u.log.WithError(e).Warningf("Failed to close %v.", fileName)
		}
		return trace.Wrap(err)
	}

	upload := &upload{
		sessionID: sessionID,
		reader:    events.NewProtoReader(sessionFile),
		file:      sessionFile,
	}
	upload.checkpointFile, err = os.OpenFile(u.checkpointFilePath(sessionID), os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		if err := upload.Close(); err != nil {
			u.log.WithError(err).Warningf("Failed to close upload.")
		}
		return trace.ConvertSystemError(err)
	}

	start := time.Now()
	if err := u.takeSemaphore(); err != nil {
		if err := upload.Close(); err != nil {
			u.log.WithError(err).Warningf("Failed to close upload.")
		}
		return trace.Wrap(err)
	}
	u.log.Debugf("Semaphore acquired in %v for upload %v.", time.Since(start), fileName)
	go func() {
		if err := u.upload(upload); err != nil {
			u.log.WithError(err).Warningf("Upload failed.")
			u.emitEvent(events.UploadEvent{
				SessionID: string(upload.sessionID),
				Error:     err,
			})
			return
		}
		u.emitEvent(events.UploadEvent{
			SessionID: string(upload.sessionID),
		})

	}()
	return nil
}

func (u *Uploader) upload(up *upload) error {
	defer u.releaseSemaphore()
	defer func() {
		if err := up.Close(); err != nil {
			u.log.WithError(err).Warningf("Failed to close upload.")
		}
	}()

	var stream events.Stream
	status, err := up.readStatus()
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		u.log.Debugf("Starting upload for session %v.", up.sessionID)
		stream, err = u.cfg.Streamer.CreateAuditStream(u.ctx, up.sessionID)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		u.log.Debugf("Resuming upload for session %v, upload ID %v.", up.sessionID, status.UploadID)
		stream, err = u.cfg.Streamer.ResumeAuditStream(u.ctx, up.sessionID, status.UploadID)
		if err != nil {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
			u.log.WithError(err).Warningf(
				"Upload for sesion %v, upload ID %v is not found starting a new upload from scratch.",
				up.sessionID, status.UploadID)
			status = nil
			stream, err = u.cfg.Streamer.CreateAuditStream(u.ctx, up.sessionID)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}

	defer func() {
		if err := stream.Close(u.ctx); err != nil {
			if trace.Unwrap(err) != io.EOF {
				u.log.WithError(err).Debugf("Failed to close stream.")
			}
		}
	}()

	// The call to CreateAuditStream is async. To learn
	// if it was successful get the first status update
	// sent by the server after create.
	select {
	case <-stream.Status():
	case <-time.After(defaults.NetworkRetryDuration):
		return trace.ConnectionProblem(nil, "timeout waiting for stream status update")
	case <-u.ctx.Done():
		return trace.ConnectionProblem(u.ctx.Err(), "operation has been cancelled")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go u.monitorStreamStatus(u.ctx, up, stream, cancel)

	start := u.cfg.Clock.Now().UTC()
	for {
		event, err := up.reader.Read(ctx)
		if err != nil {
			if err == io.EOF {
				break
			}
			return trace.Wrap(err)
		}
		// skip events that have been already submitted
		if status != nil && event.GetIndex() <= status.LastEventIndex {
			continue
		}
		if err := stream.EmitAuditEvent(u.ctx, event); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := stream.Complete(u.ctx); err != nil {
		u.log.WithError(err).Errorf("Failed to complete upload.")
		return trace.Wrap(err)
	}

	// make sure that checkpoint writer goroutine finishes
	// before the files are closed to avoid async writes
	// the timeout is a defensive measure to avoid blocking
	// indefinitely in case of unforeseen error (e.g. write taking too long)
	wctx, wcancel := context.WithTimeout(ctx, defaults.DefaultDialTimeout)
	defer wcancel()

	<-wctx.Done()
	if errors.Is(wctx.Err(), context.DeadlineExceeded) {
		u.log.WithError(wctx.Err()).Warningf(
			"Checkpoint function failed to complete the write due to timeout. Possible slow disk write.")
	}

	u.log.WithFields(log.Fields{"duration": u.cfg.Clock.Since(start), "session-id": up.sessionID}).Infof("Session upload completed.")
	// In linux it is possible to remove a file while holding a file descriptor
	if err := up.removeFiles(); err != nil {
		u.log.WithError(err).Warningf("Failed to remove session files.")
	}
	return nil
}

// monitorStreamStatus monitors stream's status
// and checkpoints the stream
func (u *Uploader) monitorStreamStatus(ctx context.Context, up *upload, stream events.Stream, cancel context.CancelFunc) {
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stream.Done():
			return
		case status := <-stream.Status():
			if err := up.writeStatus(status); err != nil {
				u.log.WithError(err).Debugf("Got stream status: %v.", status)
			} else {
				u.log.Debugf("Got stream status: %v.", status)
			}
		}
	}
}

var errContext = fmt.Errorf("context has closed")

func (u *Uploader) takeSemaphore() error {
	select {
	case u.semaphore <- struct{}{}:
		return nil
	case <-u.ctx.Done():
		return errContext
	}
}

func (u *Uploader) releaseSemaphore() error {
	select {
	case <-u.semaphore:
		return nil
	case <-u.ctx.Done():
		return errContext
	}
}

func (u *Uploader) emitEvent(e events.UploadEvent) {
	if u.cfg.EventsC == nil {
		return
	}
	select {
	case u.cfg.EventsC <- e:
		return
	default:
		u.log.Warningf("Skip send event on a blocked channel.")
	}
}
