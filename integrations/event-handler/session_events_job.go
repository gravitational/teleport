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
	"os"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"

	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/backoff"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

const (
	// sessionBackoffBase is an initial (minimum) backoff value.
	sessionBackoffBase = 3 * time.Second
	// sessionBackoffMax is a backoff threshold
	sessionBackoffMax = 2 * time.Minute
	// sessionBackoffNumTries is the maximum number of backoff tries
	sessionBackoffNumTries = 5
)

// session is the utility struct used for session ingestion
type session struct {
	// ID current ID
	ID string
	// Index current event index
	Index int64
}

// SessionEventsJob incapsulates session events consumption logic
type SessionEventsJob struct {
	lib.ServiceJob
	app       *App
	sessions  chan session
	semaphore *semaphore.Weighted
}

// NewSessionEventsJob creates new EventsJob structure
func NewSessionEventsJob(app *App) *SessionEventsJob {
	j := &SessionEventsJob{
		app:       app,
		semaphore: semaphore.NewWeighted(int64(app.Config.Concurrency)),
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
			if err := j.semaphore.Acquire(ctx, 1); err != nil {
				log.WithError(err).Error("Failed to acquire semaphore")
				continue
			}

			log.WithField("id", s.ID).WithField("index", s.Index).Info("Starting session ingest")

			func(s session) {
				j.app.SpawnCritical(func(ctx context.Context) error {
					defer j.semaphore.Release(1)

					backoff := backoff.NewDecorr(sessionBackoffBase, sessionBackoffMax, clockwork.NewRealClock())
					backoffCount := sessionBackoffNumTries
					log := logger.Get(ctx).WithField("id", s.ID).WithField("index", s.Index)

					for {
						retry, err := j.consumeSession(ctx, s)

						// If sessions needs to retry
						if err != nil && retry {
							log.WithError(err).WithField("n", backoffCount).Error("Session ingestion error, retrying")

							// Sleep for required interval
							err := backoff.Do(ctx)
							if err != nil {
								return trace.Wrap(err)
							}

							// Check if there are number of tries left
							backoffCount--
							if backoffCount < 0 {
								log.WithField("err", err).Error("Session ingestion failed")
								return nil
							}
							continue
						}

						if err != nil {
							if !lib.IsCanceled(err) {
								log.WithField("err", err).Error("Session ingestion failed")
							}
							return err
						}

						// No errors, we've finished with this session
						return nil
					}
				})
			}(s)
		case <-ctx.Done():
			if lib.IsCanceled(ctx.Err()) {
				return nil
			}
			return ctx.Err()
		}
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
				log.WithField("id", id).WithField("index", idx).Info("Restarting session ingestion")

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
	log := logger.Get(ctx)

	url := j.app.Config.FluentdSessionURL + "." + s.ID + ".log"

	log.WithField("id", s.ID).WithField("index", s.Index).Info("Started session events ingest")
	chEvt, chErr := j.app.EventWatcher.StreamUnstructuredSessionEvents(ctx, s.ID, s.Index)

Loop:
	for {
		select {
		case err := <-chErr:
			return true, trace.Wrap(err)

		case evt, ok := <-chEvt:
			if !ok {
				log.WithField("id", s.ID).Info("Finished session events ingest")
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
	// If the session had no events, the file won't exist, so we ignore the error
	if err != nil && !os.IsNotExist(err) {
		return false, trace.Wrap(err)
	}

	return false, nil
}

// Register starts session event ingestion
func (j *SessionEventsJob) RegisterSession(ctx context.Context, e *TeleportEvent) error {
	err := j.app.State.SetSessionIndex(e.SessionID, 0)
	if err != nil {
		return trace.Wrap(err)
	}

	s := session{ID: e.SessionID, Index: 0}

	go func() {
		select {
		case j.sessions <- s:
			return
		case <-ctx.Done():
			log.Error(ctx.Err())
			return
		}
	}()

	return nil
}
