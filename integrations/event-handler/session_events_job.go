// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/backoff"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

// session is the utility struct used for session ingestion
type session struct {
	// ID current ID
	ID string
	// Index current event index
	Index int64
	// UploadTime is the time at which the recording was uploaded.
	UploadTime time.Time
}

// SessionEventsJob incapsulates session events consumption logic
type SessionEventsJob struct {
	lib.ServiceJob
	app       *App
	sessions  chan session
	semaphore chan struct{}
}

// NewSessionEventsJob creates new EventsJob structure
func NewSessionEventsJob(app *App) *SessionEventsJob {
	j := &SessionEventsJob{
		app:       app,
		semaphore: make(chan struct{}, app.Config.Concurrency),
		sessions:  make(chan session),
	}

	j.ServiceJob = lib.NewServiceJob(j.run)

	return j
}

// run runs session consuming process
func (j *SessionEventsJob) run(ctx context.Context) error {
	log := logger.Get(ctx)

	// Create cancellable context which handles app termination
	process := lib.MustGetProcess(ctx)
	ctx, cancel := context.WithCancel(ctx)
	process.OnTerminate(func(_ context.Context) error {
		cancel()
		return nil
	})

	if err := j.restartPausedSessions(); err != nil {
		log.WithError(err).Error("Restarting paused sessions")
	}

	j.SetReady(true)

	for {
		select {
		case s := <-j.sessions:
			logger := log.WithField("id", s.ID).WithField("index", s.Index)

			logger.Info("Starting session ingest")

			select {
			case j.semaphore <- struct{}{}:
			case <-ctx.Done():
				logger.WithError(ctx.Err()).Error("Failed to acquire semaphore")
				return nil
			}

			func(s session, log logrus.FieldLogger) {
				j.app.SpawnCritical(func(ctx context.Context) error {
					defer func() { <-j.semaphore }()

					if err := j.processSession(ctx, s, 0); err != nil {
						return trace.Wrap(err)
					}

					return nil
				})
			}(s, logger)
		case <-ctx.Done():
			if lib.IsCanceled(ctx.Err()) {
				return nil
			}
			return ctx.Err()
		}
	}
}

func (j *SessionEventsJob) processSession(ctx context.Context, s session, processingAttempt int) error {
	const (
		// maxNumberOfProcessingAttempts is the number of times a non-existent
		// session recording will be processed before assuming the recording
		// is lost forever.
		maxNumberOfProcessingAttempts = 2
		// eventTimeCutOff is the point of time in the past at which a missing
		// session recording is assumed to be lost forever.
		eventTimeCutOff = -48 * time.Hour
		// sessionBackoffBase is an initial (minimum) backoff value.
		sessionBackoffBase = 3 * time.Second
		// sessionBackoffMax is a backoff threshold
		sessionBackoffMax = time.Minute
		// sessionBackoffNumTries is the maximum number of backoff tries
		sessionBackoffNumTries = 3
	)

	log := logger.Get(ctx).WithField("id", s.ID).WithField("index", s.Index)
	backoff := backoff.NewDecorr(sessionBackoffBase, sessionBackoffMax, clockwork.NewRealClock())
	attempt := sessionBackoffNumTries

	for {
		retry, err := j.consumeSession(ctx, s)
		switch {
		case trace.IsNotFound(err):
			// If the session was not found, and it was from more
			// than the [eventTimeCutOff], abort processing it any further
			// as the recording is likely never going to exist.
			if (!s.UploadTime.IsZero() && s.UploadTime.Before(time.Now().Add(eventTimeCutOff))) ||
				processingAttempt > maxNumberOfProcessingAttempts {
				// Remove any trace of the session so that it is not
				// processed again in the future and doesn't stick around
				// on disk forever.
				return trace.NewAggregate(j.app.State.RemoveMissingRecording(s.ID), j.app.State.RemoveSession(s.ID))
			}

			// Otherwise, mark that the session was failed to be processed
			// so that it can be tried again in the background.
			return trace.NewAggregate(j.app.State.SetMissingRecording(s, processingAttempt+1), j.app.State.RemoveSession(s.ID))
		case err != nil && retry:
			// If the number of retries has been exceeded, then
			// abort processing the session any further.
			attempt--
			if attempt <= 0 {
				log.WithField("limit", sessionBackoffNumTries).Error("Session ingestion exceeded attempt limit, aborting")
				return trace.LimitExceeded("Session ingestion exceeded attempt limit")
			}

			log.WithError(err).WithField("n", attempt).Error("Session ingestion error, retrying")

			// Perform backoff before retrying the session again.
			if err := backoff.Do(ctx); err != nil {
				return trace.Wrap(err)
			}
		case err != nil:
			// Abort on any errors that don't require a retry.
			if !lib.IsCanceled(err) {
				log.WithField("err", err).Error("Session ingestion failed")
			}
			return trace.Wrap(err)
		default:
			// No errors, we've finished processing the session.
			return nil
		}
	}
}

// processMissingRecordings periodically attempts to reconcile events
// from session recordings that were previously not found.
func (j *SessionEventsJob) processMissingRecordings(ctx context.Context) error {
	const (
		initialProcessingDelay      = time.Minute
		processingInterval          = 3 * time.Minute
		maxNumberOfInflightSessions = 10
	)

	ctx, cancel := context.WithCancel(ctx)
	j.app.Process.OnTerminate(func(_ context.Context) error {
		cancel()
		return nil
	})

	jitter := retryutils.NewSeventhJitter()
	timer := time.NewTimer(jitter(initialProcessingDelay))
	defer timer.Stop()

	semaphore := make(chan struct{}, maxNumberOfInflightSessions)
	for {
		select {
		case <-timer.C:
		case <-ctx.Done():
			return nil
		}

		err := j.app.State.IterateMissingRecordings(func(sess session, attempts int) error {
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				return trace.Wrap(ctx.Err())
			}

			go func() {
				defer func() { <-semaphore }()

				if err := j.processSession(ctx, sess, attempts); err != nil {
					logger.Get(ctx).WithError(err).Debug("Failed processing session recording")
				}
			}()

			return nil
		})
		if err != nil && !lib.IsCanceled(err) {
			logger.Get(ctx).WithError(err).Warn("Unable to load previously failed sessions for processing")
		}

		timer.Reset(jitter(processingInterval))
	}
}

// restartPausedSessions restarts sessions saved in state
func (j *SessionEventsJob) restartPausedSessions() error {
	sessions, err := j.app.State.GetSessions()
	if err != nil {
		return trace.Wrap(err)
	}

	if len(sessions) == 0 {
		return nil
	}

	for id, idx := range sessions {
		func(id string, idx int64) {
			j.app.SpawnCritical(func(ctx context.Context) error {
				logrus.WithField("id", id).WithField("index", idx).Info("Restarting session ingestion")

				s := session{ID: id, Index: idx}

				select {
				case j.sessions <- s:
					return nil
				case <-ctx.Done():
					if lib.IsCanceled(ctx.Err()) {
						return nil
					}

					return ctx.Err()
				}
			})
		}(id, idx)
	}

	return nil
}

// consumeSession ingests session
func (j *SessionEventsJob) consumeSession(ctx context.Context, s session) (bool, error) {
	url := j.app.Config.FluentdSessionURL + "." + s.ID + ".log"
	chEvt, chErr := j.app.EventWatcher.StreamUnstructuredSessionEvents(ctx, s.ID, s.Index)

Loop:
	for {
		select {
		case err := <-chErr:
			return true, trace.Wrap(err)

		case evt, ok := <-chEvt:
			if !ok {
				logrus.WithField("id", s.ID).Info("Finished session events ingest")
				break Loop // Break the main loop
			}

			e, err := NewTeleportEvent(evt, "")
			if err != nil {
				return false, trace.Wrap(err)
			}

			_, ok = j.app.Config.SkipSessionTypes[e.Type]
			if !ok {
				err := j.app.SendEvent(ctx, url, e)

				if err != nil && trace.IsConnectionProblem(err) {
					return true, trace.Wrap(err)
				}
				if err != nil {
					return false, trace.Wrap(err)
				}
			}

			// Set session index
			err = j.app.State.SetSessionIndex(s.ID, e.Index)
			if err != nil {
				return true, trace.Wrap(err)
			}
		case <-ctx.Done():
			if lib.IsCanceled(ctx.Err()) {
				return false, nil
			}

			return false, trace.Wrap(ctx.Err())
		}
	}

	// We have finished ingestion and do not need session state anymore
	err := j.app.State.RemoveSession(s.ID)
	return false, trace.Wrap(err)
}

// RegisterSession starts session event ingestion
func (j *SessionEventsJob) RegisterSession(ctx context.Context, e *TeleportEvent) error {
	err := j.app.State.SetSessionIndex(e.SessionID, 0)
	if err != nil {
		return trace.Wrap(err)
	}

	s := session{ID: e.SessionID, Index: 0, UploadTime: e.Time}

	go func() {
		select {
		case j.sessions <- s:
			return
		case <-ctx.Done():
			if !lib.IsCanceled(ctx.Err()) {
				logrus.Error(ctx.Err())
			}
			return
		}
	}()

	return nil
}
