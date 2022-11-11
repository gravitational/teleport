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
	"time"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
)

// UploadCompleterConfig specifies configuration for the uploader
type UploadCompleterConfig struct {
	// AuditLog is used for storing logs
	AuditLog IAuditLog
	// Uploader allows the completer to list and complete uploads
	Uploader MultipartUploader
	// GracePeriod is the period after which uploads are considered
	// abandoned and will be completed
	GracePeriod time.Duration
	// Component is a component used in logging
	Component string
	// CheckPeriod is a period for checking the upload
	CheckPeriod time.Duration
	// Clock is used to override clock in tests
	Clock clockwork.Clock
	// Unstarted does not start automatic goroutine,
	// is useful when completer is embedded in another function
	Unstarted bool
}

// CheckAndSetDefaults checks and sets default values
func (cfg *UploadCompleterConfig) CheckAndSetDefaults() error {
	if cfg.Uploader == nil {
		return trace.BadParameter("missing parameter Uploader")
	}
	if cfg.GracePeriod == 0 {
		cfg.GracePeriod = defaults.UploadGracePeriod
	}
	if cfg.Component == "" {
		cfg.Component = teleport.ComponentAuth
	}
	if cfg.CheckPeriod == 0 {
		cfg.CheckPeriod = defaults.LowResPollingPeriod
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	return nil
}

// NewUploadCompleter returns a new instance of the upload completer
// the completer has to be closed to release resources and goroutines
func NewUploadCompleter(cfg UploadCompleterConfig) (*UploadCompleter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	u := &UploadCompleter{
		cfg: cfg,
		log: log.WithFields(log.Fields{
			trace.Component: teleport.Component(cfg.Component, "completer"),
		}),
		cancel:   cancel,
		closeCtx: ctx,
	}
	if !cfg.Unstarted {
		go u.run()
	}
	return u, nil
}

// UploadCompleter periodically scans uploads that have not been completed
// and completes them
type UploadCompleter struct {
	cfg      UploadCompleterConfig
	log      *log.Entry
	cancel   context.CancelFunc
	closeCtx context.Context
}

func (u *UploadCompleter) run() {
	periodic := interval.New(interval.Config{
		Duration:      u.cfg.CheckPeriod,
		FirstDuration: utils.HalfJitter(u.cfg.CheckPeriod),
		Jitter:        utils.NewSeventhJitter(),
	})
	defer periodic.Stop()

	for {
		select {
		case <-periodic.Next():
			if err := u.CheckUploads(u.closeCtx); err != nil {
				u.log.WithError(err).Warningf("Failed to check uploads.")
			}
		case <-u.closeCtx.Done():
			return
		}
	}
}

// CheckUploads fetches uploads, checks if any uploads exceed grace period
// and completes unfinished uploads
func (u *UploadCompleter) CheckUploads(ctx context.Context) error {
	uploads, err := u.cfg.Uploader.ListUploads(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	completed := 0
	for _, upload := range uploads {
		gracePoint := upload.Initiated.Add(u.cfg.GracePeriod)
		if !gracePoint.Before(u.cfg.Clock.Now()) {
			return nil
		}
		parts, err := u.cfg.Uploader.ListParts(ctx, upload)
		if err != nil {
			if trace.IsNotFound(err) {
				u.log.WithError(err).Warnf("Missing parts for upload %v. Moving on to next upload.", upload.ID)
				continue
			}
			return trace.Wrap(err)
		}

		u.log.Debugf("Upload %v grace period is over. Trying to complete.", upload.ID)
		if err := u.cfg.Uploader.CompleteUpload(ctx, upload, parts); err != nil {
			return trace.Wrap(err)
		}
		u.log.Debugf("Completed upload %v.", upload)
		completed++

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
			continue
		}

		// Schedule a background operation to check for (and emit) a session end event.
		// This is necessary because we'll need to download the session in order to
		// enumerate its events, and the S3 API takes a little while after the upload
		// is completed before version metadata becomes available.
		upload := upload // capture range variable
		go func() {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Minute):
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
				Time:  u.cfg.Clock.Now().UTC(),
				ID:    uuid.New(),
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
	if completed > 0 {
		u.log.Debugf("Found %v active uploads, completed %v.", len(uploads), completed)
	}
	return nil
}

// Close closes all outstanding operations without waiting
func (u *UploadCompleter) Close() error {
	u.cancel()
	return nil
}

func (u *UploadCompleter) ensureSessionEndEvent(ctx context.Context, uploadData UploadMetadata) error {
	var serverID, clusterName, user, login, hostname, namespace, serverAddr string
	var interactive bool

	// Get session events to find fields for constructed session end
	sessionEvents, err := u.cfg.AuditLog.GetSessionEvents(apidefaults.Namespace, uploadData.SessionID, 0, false)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(sessionEvents) == 0 {
		return nil
	}

	// Return if session.end event already exists
	for _, event := range sessionEvents {
		if event.GetType() == SessionEndEvent {
			return nil
		}
	}

	// Session start event is the first of session events
	sessionStart := sessionEvents[0]
	if sessionStart.GetType() != SessionStartEvent {
		return trace.BadParameter("invalid session, session start is not the first event")
	}

	// Set variables
	serverID = sessionStart.GetString(SessionServerHostname)
	clusterName = sessionStart.GetString(SessionClusterName)
	hostname = sessionStart.GetString(SessionServerHostname)
	namespace = sessionStart.GetString(EventNamespace)
	serverAddr = sessionStart.GetString(SessionServerAddr)
	user = sessionStart.GetString(EventUser)
	login = sessionStart.GetString(EventLogin)
	if terminalSize := sessionStart.GetString(TerminalSize); terminalSize != "" {
		interactive = true
	}

	// Get last event to get session end time
	lastEvent := sessionEvents[len(sessionEvents)-1]

	participants := getParticipants(sessionEvents)

	sessionEndEvent := &events.SessionEnd{
		Metadata: events.Metadata{
			Type:        SessionEndEvent,
			Code:        SessionEndCode,
			ClusterName: clusterName,
		},
		ServerMetadata: events.ServerMetadata{
			ServerID:        serverID,
			ServerNamespace: namespace,
			ServerHostname:  hostname,
			ServerAddr:      serverAddr,
		},
		SessionMetadata: events.SessionMetadata{
			SessionID: string(uploadData.SessionID),
		},
		UserMetadata: events.UserMetadata{
			User:  user,
			Login: login,
		},
		Participants: participants,
		Interactive:  interactive,
		StartTime:    sessionStart.GetTime(EventTime),
		EndTime:      lastEvent.GetTime(EventTime),
	}

	// Check and set event fields
	if err = checkAndSetEventFields(sessionEndEvent, u.cfg.Clock, utils.NewRealUID(), clusterName); err != nil {
		return trace.Wrap(err)
	}
	if err = u.cfg.AuditLog.EmitAuditEvent(ctx, sessionEndEvent); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func getParticipants(sessionEvents []EventFields) []string {
	var participants []string
	for _, event := range sessionEvents {
		if event.GetType() == SessionJoinEvent || event.GetType() == SessionStartEvent {
			participant := event.GetString(EventUser)
			participants = append(participants, participant)

		}
	}
	return apiutils.Deduplicate(participants)
}
