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

package events

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

// UploadCompleterConfig specifies configuration for the uploader
type UploadCompleterConfig struct {
	// AuditLog is used for storing logs
	AuditLog AuditLogSessionStreamer
	// Uploader allows the completer to list and complete uploads
	Uploader MultipartUploader
	// SessionTracker is used to discover the current state of a
	// sesssions with active uploads.
	SessionTracker services.SessionTrackerService
	// Semaphores is used to optionally acquire a semaphore prior to completing
	// uploads. When specified, ServerID must also be provided.
	Semaphores types.Semaphores
	// ServerID identifies the server running the upload completer.
	ServerID string
	// Component is a component used in logging
	Component string
	// CheckPeriod is a period for checking the upload
	CheckPeriod time.Duration
	// Clock is used to override clock in tests
	Clock clockwork.Clock
	// ClusterName identifies the originating teleport cluster
	ClusterName string
}

// CheckAndSetDefaults checks and sets default values
func (cfg *UploadCompleterConfig) CheckAndSetDefaults() error {
	if cfg.Uploader == nil {
		return trace.BadParameter("missing parameter Uploader")
	}
	if cfg.SessionTracker == nil {
		return trace.BadParameter("missing parameter SessionTracker")
	}
	if cfg.ClusterName == "" {
		return trace.BadParameter("missing parameter ClusterName")
	}
	if cfg.Semaphores != nil && cfg.ServerID == "" {
		return trace.BadParameter("a server ID must be specified in order to use semaphores")
	}
	if cfg.Component == "" {
		cfg.Component = teleport.ComponentProcess
	}
	if cfg.CheckPeriod == 0 {
		cfg.CheckPeriod = AbandonedUploadPollingRate
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	return nil
}

var incompleteSessionUploads = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Namespace: "teleport",
		Name:      teleport.MetricIncompleteSessionUploads,
		Help:      "Number of sessions not yet uploaded to auth",
	},
)

// NewUploadCompleter returns a new UploadCompleter.
func NewUploadCompleter(cfg UploadCompleterConfig) (*UploadCompleter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	u := &UploadCompleter{
		cfg: cfg,
		log: log.WithFields(log.Fields{
			teleport.ComponentKey: teleport.Component(cfg.Component, "completer"),
		}),
		closeC: make(chan struct{}),
	}

	err := metrics.RegisterPrometheusCollectors(incompleteSessionUploads)
	if err != nil {
		return nil, trace.Wrap(err)
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

const (
	semaphoreName      = "upload-completer"
	semaphoreMaxLeases = 1 // allow one upload completer to operate at a time
)

// Serve runs the upload completer until closed or until ctx is canceled.
func (u *UploadCompleter) Serve(ctx context.Context) error {
	periodic := interval.New(interval.Config{
		Clock:         u.cfg.Clock,
		Duration:      u.cfg.CheckPeriod,
		FirstDuration: utils.HalfJitter(u.cfg.CheckPeriod),
		Jitter:        retryutils.NewSeventhJitter(),
	})
	defer periodic.Stop()
	u.log.Infof("upload completer will run every %v", u.cfg.CheckPeriod.String())

	for {
		select {
		case <-periodic.Next():
			u.PerformPeriodicCheck(ctx)
		case <-u.closeC:
			return nil
		case <-ctx.Done():
			return nil
		}
	}
}

func (u *UploadCompleter) PerformPeriodicCheck(ctx context.Context) {
	// If configured with a server ID, then acquire a semaphore prior to completing uploads.
	// This is used for auth's upload completer and ensures that multiple auth servers do not
	// attempt to complete the same uploads at the same time.
	// TODO(zmb3): remove the env var check once the semaphore is proven to be reliable
	if u.cfg.Semaphores != nil && os.Getenv("TELEPORT_DISABLE_UPLOAD_COMPLETER_SEMAPHORE") == "" {
		u.log.Debugf("%v: acquiring semaphore in order to complete uploads", u.cfg.ServerID)

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		_, err := services.AcquireSemaphoreLock(ctx, services.SemaphoreLockConfig{
			Service: u.cfg.Semaphores,
			Clock:   u.cfg.Clock,
			Expiry:  (u.cfg.CheckPeriod / 2) + 1,
			Params: types.AcquireSemaphoreRequest{
				SemaphoreKind: types.SemaphoreKindUploadCompleter,
				SemaphoreName: semaphoreName,
				MaxLeases:     semaphoreMaxLeases,
				Holder:        u.cfg.ServerID,
			},
		})
		if err != nil {
			u.log.Debugf("%v did not acquire semaphore, will skip this round of uploads", u.cfg.ServerID)
			return
		}
	}
	if err := u.CheckUploads(ctx); trace.IsAccessDenied(err) {
		u.log.Warn("Teleport does not have permission to list uploads. " +
			"The upload completer will be unable to complete uploads of partial session recordings.")
	} else if err != nil {
		u.log.WithError(err).Warn("Failed to check uploads.")
	}
}

// CheckUploads fetches uploads and completes any abandoned uploads
func (u *UploadCompleter) CheckUploads(ctx context.Context) error {
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

	incompleteSessionUploads.Set(float64(len(uploads)))
	// Complete upload for any uploads without an active session tracker
	for _, upload := range uploads {
		log := u.log.WithField("upload", upload.ID).WithField("session", upload.SessionID)

		switch _, err := u.cfg.SessionTracker.GetSessionTracker(ctx, upload.SessionID.String()); {
		case err == nil: // session is still in progress, continue to other uploads
			log.Debug("session has active tracker and is not ready to be uploaded")
			continue
		case trace.IsNotFound(err): // upload abandoned, complete upload
		default: // aka err != nil
			log.Warn("could not get session tracker, skipping upload")
			continue
		}

		log.Debug("Upload was abandoned, trying to complete")

		parts, err := u.cfg.Uploader.ListParts(ctx, upload)
		if err != nil {
			if trace.IsNotFound(err) {
				log.WithError(err).Warn("Missing parts, moving on to next upload")
				incompleteSessionUploads.Dec()
				continue
			}
			return trace.Wrap(err, "listing parts")
		}

		log.Debugf("upload has %d parts", len(parts))

		if err := u.cfg.Uploader.CompleteUpload(ctx, upload, parts); trace.IsNotFound(err) {
			log.WithError(err).Debug("Upload not found, moving on to next upload")
			continue
		} else if err != nil {
			return trace.Wrap(err, "completing upload %v for session %v", upload.ID, upload.SessionID)
		}
		log.Debug("Completed upload")
		completed++
		incompleteSessionUploads.Dec()

		if len(parts) == 0 {
			continue
		}

		uploadData := u.cfg.Uploader.GetUploadMetadata(upload.SessionID)

		// It's possible that we don't have a session ID here. For example,
		// an S3 multipart upload may have been completed by another auth
		// server, in which case the API returns an empty key, leaving us
		// no way to derive the session ID from the upload.
		//
		// If this is the case, there's no work left to do, and we can
		// proceed to the next upload.
		if uploadData.SessionID == "" {
			log.Debug("No session ID in metadata skipping session end check")
			continue
		}

		// Schedule a background operation to check for (and emit) a session end event.
		// This is necessary because we'll need to download the session in order to
		// enumerate its events, and the S3 API takes a little while after the upload
		// is completed before version metadata becomes available.
		go func() {
			select {
			case <-ctx.Done():
				return
			case <-u.cfg.Clock.After(2 * time.Minute):
				log.Debug("checking for session end event")
				if err := u.ensureSessionEndEvent(ctx, uploadData); err != nil {
					log.WithError(err).Warning("failed to ensure session end event")
				}
			}
		}()
		session := &events.SessionUpload{
			Metadata: events.Metadata{
				Type:        SessionUploadEvent,
				Code:        SessionUploadCode,
				Time:        u.cfg.Clock.Now().UTC(),
				ID:          uuid.New().String(),
				Index:       SessionUploadIndex,
				ClusterName: u.cfg.ClusterName,
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

	sessionEndEvent.SetTime(lastEvent.GetTime())

	// Check and set event fields
	if err := checkAndSetEventFields(sessionEndEvent, u.cfg.Clock, utils.NewRealUID(), sessionEndEvent.GetClusterName()); err != nil {
		return trace.Wrap(err)
	}
	if err := u.cfg.AuditLog.EmitAuditEvent(ctx, sessionEndEvent); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
