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

package events

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"

	"github.com/gravitational/trace"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

// UploadCompleterConfig specifies configuration for the uploader
type UploadCompleterConfig struct {
	// AuditLog is used for storing logs
	AuditLog IAuditLog
	// Uploader allows the completer to list and complete uploads
	Uploader MultipartUploader
	// SessionTracker is used to discover the current state of a
	// sesssions with active uploads.
	SessionTracker services.SessionTrackerService
	// Component is a component used in logging
	Component string
	// CheckPeriod is a period for checking the upload
	CheckPeriod time.Duration
	// Clock is used to override clock in tests
	Clock clockwork.Clock
	// DELETE IN 11.0.0 in favor of SessionTrackerService
	// GracePeriod is the period after which an upload's session
	// tracker will be check to see if it's an abandoned upload.
	GracePeriod time.Duration
}

// CheckAndSetDefaults checks and sets default values
func (cfg *UploadCompleterConfig) CheckAndSetDefaults() error {
	if cfg.Uploader == nil {
		return trace.BadParameter("missing parameter Uploader")
	}
	if cfg.SessionTracker == nil {
		return trace.BadParameter("missing parameter SessionTracker")
	}
	if cfg.Component == "" {
		cfg.Component = teleport.ComponentAuth
	}
	if cfg.CheckPeriod == 0 {
		cfg.CheckPeriod = defaults.AbandonedUploadPollingRate
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	return nil
}

// NewUploadCompleter returns a new UploadCompleter.
func NewUploadCompleter(cfg UploadCompleterConfig) (*UploadCompleter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	u := &UploadCompleter{
		cfg: cfg,
		log: log.WithFields(log.Fields{
			trace.Component: teleport.Component(cfg.Component, "completer"),
		}),
		closeC: make(chan struct{}),
	}
	return u, nil
}

// StartNewUploadCompleter starts an upload completer background process that will
// will close once the provided ctx is closed.
func StartNewUploadCompleter(ctx context.Context, cfg UploadCompleterConfig) error {
	uc, err := NewUploadCompleter(cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	go uc.Serve(ctx)
	return nil
}

// UploadCompleter periodically scans uploads that have not been completed
// and completes them
type UploadCompleter struct {
	cfg    UploadCompleterConfig
	log    *log.Entry
	closeC chan struct{}
}

// Close stops the UploadCompleter
func (u *UploadCompleter) Close() {
	close(u.closeC)
}

// Serve runs the upload completer until closed or until ctx is canceled.
func (u *UploadCompleter) Serve(ctx context.Context) error {
	periodic := interval.New(interval.Config{
		Duration:      u.cfg.CheckPeriod,
		FirstDuration: utils.HalfJitter(u.cfg.CheckPeriod),
		Jitter:        utils.NewSeventhJitter(),
	})
	defer periodic.Stop()

	for {
		select {
		case <-periodic.Next():
			if err := u.checkUploads(ctx); err != nil {
				u.log.WithError(err).Warningf("Failed to check uploads.")
			}
		case <-u.closeC:
			return nil
		case <-ctx.Done():
			return nil
		}
	}
}

// checkUploads fetches uploads and completes any abandoned uploads
func (u *UploadCompleter) checkUploads(ctx context.Context) error {
	uploads, err := u.cfg.Uploader.ListUploads(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	completed := 0
	defer func() {
		if completed > 0 {
			u.log.Debugf("Found %v active uploads, completed %v.", len(uploads), completed)
		}
	}()

	// Complete upload for any uploads without an active session tracker
	for _, upload := range uploads {
		// DELETE IN 11.0.0
		// To support v9/v8 versions which do not use SessionTrackerService,
		// sessions are only considered abandoned after the provided grace period.
		if u.cfg.GracePeriod != 0 {
			gracePoint := upload.Initiated.Add(u.cfg.GracePeriod)
			if gracePoint.After(u.cfg.Clock.Now()) {
				// uploads are ordered oldest to newest, stop checking
				// once an upload doesn't exceed the grace point
				return nil
			}
		}

		switch _, err := u.cfg.SessionTracker.GetSessionTracker(ctx, upload.SessionID.String()); {
		case err == nil: // session is still in progress, continue to other uploads
			continue
		case trace.IsNotFound(err): // upload abandoned, complete upload
		case trace.IsAccessDenied(err): // upload abandoned, complete upload
			// Treat access denied errors as not found errors, since we expect
			// to get them if the auth server is v9.2.3 or earlier, since only
			// node, proxy, and kube roles had permissions to create trackers.
			// DELETE IN 11.0.0
		default: // aka err != nil
			return trace.Wrap(err)
		}

		parts, err := u.cfg.Uploader.ListParts(ctx, upload)
		if err != nil {
			if trace.IsNotFound(err) {
				u.log.WithError(err).Warnf("Missing parts for upload %v. Moving on to next upload.", upload.ID)
				continue
			}
			return trace.Wrap(err)
		}

		u.log.Debugf("Upload %v was abandoned, trying to complete.", upload.ID)
		if err := u.cfg.Uploader.CompleteUpload(ctx, upload, parts); err != nil {
			return trace.Wrap(err)
		}
		u.log.Debugf("Completed upload %v.", upload)
		completed++

		if len(parts) == 0 {
			continue
		}

		uploadData := u.cfg.Uploader.GetUploadMetadata(upload.SessionID)

		// Schedule a background operation to check for (and emit) a session end event.
		// This is necessary because we'll need to download the session in order to
		// enumerate its events, and the S3 API takes a little while after the upload
		// is completed before version metadata becomes available.
		go func() {
			select {
			case <-ctx.Done():
				return
			case <-u.cfg.Clock.After(2 * time.Minute):
				u.log.Debugf("checking for session end event for session %v", upload.SessionID)
				if err := u.ensureSessionEndEvent(ctx, uploadData); err != nil {
					u.log.WithError(err).Warningf("failed to ensure session end event for session %v", upload.SessionID)
				}
			}
		}()
		session := &events.SessionUpload{
			Metadata: events.Metadata{
				Type:  SessionUploadEvent,
				Code:  SessionUploadCode,
				ID:    uuid.New().String(),
				Index: SessionUploadIndex,
			},
			SessionMetadata: events.SessionMetadata{
				SessionID: string(uploadData.SessionID),
			},
			SessionURL: uploadData.URL,
		}
		err = u.cfg.AuditLog.EmitAuditEvent(ctx, session)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (u *UploadCompleter) ensureSessionEndEvent(ctx context.Context, uploadData UploadMetadata) error {
	// at this point, we don't know whether we'll need to emit a session.end or a
	// windows.desktop.session.end, but as soon as we see the session start we'll
	// be able to start filling in the details
	var sshSessionEnd events.SessionEnd
	var desktopSessionEnd events.WindowsDesktopSessionEnd

	// We use the streaming events API to search through the session events, because it works
	// for both Desktop and SSH sessions, where as the GetSessionEvents API relies on downloading
	// a copy of the session and using the SSH-specific index to iterate through events.
	var lastEvent events.AuditEvent
	evts, errors := u.cfg.AuditLog.StreamSessionEvents(ctx, uploadData.SessionID, 0)

loop:
	for {
		select {
		case evt, more := <-evts:
			if !more {
				break loop
			}

			lastEvent = evt

			switch e := evt.(type) {
			// Return if session end event already exists
			case *events.SessionEnd, *events.WindowsDesktopSessionEnd:
				return nil

			case *events.WindowsDesktopSessionStart:
				desktopSessionEnd.Type = WindowsDesktopSessionEndEvent
				desktopSessionEnd.Code = DesktopSessionEndCode
				desktopSessionEnd.ClusterName = e.ClusterName
				desktopSessionEnd.StartTime = e.Time
				desktopSessionEnd.Participants = append(desktopSessionEnd.Participants, e.User)
				desktopSessionEnd.Recorded = true
				desktopSessionEnd.UserMetadata = e.UserMetadata
				desktopSessionEnd.SessionMetadata = e.SessionMetadata
				desktopSessionEnd.WindowsDesktopService = e.WindowsDesktopService
				desktopSessionEnd.Domain = e.Domain
				desktopSessionEnd.DesktopAddr = e.DesktopAddr
				desktopSessionEnd.DesktopLabels = e.DesktopLabels
				desktopSessionEnd.DesktopName = fmt.Sprintf("%v (recovered)", e.DesktopName)

			case *events.SessionStart:
				sshSessionEnd.Type = SessionEndEvent
				sshSessionEnd.Code = SessionEndCode
				sshSessionEnd.ClusterName = e.ClusterName
				sshSessionEnd.StartTime = e.Time
				sshSessionEnd.UserMetadata = e.UserMetadata
				sshSessionEnd.SessionMetadata = e.SessionMetadata
				sshSessionEnd.ServerMetadata = e.ServerMetadata
				sshSessionEnd.ConnectionMetadata = e.ConnectionMetadata
				sshSessionEnd.KubernetesClusterMetadata = e.KubernetesClusterMetadata
				sshSessionEnd.KubernetesPodMetadata = e.KubernetesPodMetadata
				sshSessionEnd.InitialCommand = e.InitialCommand
				sshSessionEnd.SessionRecording = e.SessionRecording
				sshSessionEnd.Interactive = e.TerminalSize != ""
				sshSessionEnd.Participants = append(sshSessionEnd.Participants, e.User)

			case *events.SessionJoin:
				sshSessionEnd.Participants = append(sshSessionEnd.Participants, e.User)
			}

		case err := <-errors:
			return trace.Wrap(err)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if lastEvent == nil {
		return trace.Errorf("could not find any events for session %v", uploadData.SessionID)
	}

	sshSessionEnd.Participants = apiutils.Deduplicate(sshSessionEnd.Participants)
	sshSessionEnd.EndTime = lastEvent.GetTime()
	desktopSessionEnd.EndTime = lastEvent.GetTime()

	var sessionEndEvent events.AuditEvent
	switch {
	case sshSessionEnd.Code != "":
		sessionEndEvent = &sshSessionEnd
	case desktopSessionEnd.Code != "":
		sessionEndEvent = &desktopSessionEnd
	default:
		return trace.BadParameter("invalid session, could not find session start")
	}

	u.log.Infof("emitting %T event for completed session %v", sessionEndEvent, uploadData.SessionID)

	// Check and set event fields
	if err := checkAndSetEventFields(sessionEndEvent, u.cfg.Clock, utils.NewRealUID(), sessionEndEvent.GetClusterName()); err != nil {
		return trace.Wrap(err)
	}
	if err := u.cfg.AuditLog.EmitAuditEvent(ctx, sessionEndEvent); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
