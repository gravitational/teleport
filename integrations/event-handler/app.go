/*
Copyright 2015-2021 Gravitational, Inc.

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

package main

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/backoff"
	"github.com/gravitational/teleport/lib/integrations/diagnostics"
)

// App is the app structure
type App struct {
	// Fluentd represents the instance of Fluentd client
	Fluentd *FluentdClient
	// State represents the instance of the persistent state
	State *State
	// cmd is start command CLI config
	Config *StartCmdConfig
	// client is the teleport api client
	client TeleportSearchEventsClient
	// eventsJob represents main audit log event consumer job
	eventsJob *EventsJob
	// sessionEventsJob represents session events consumer job
	sessionEventsJob *SessionEventsJob
	// log is the logger to use.
	log *slog.Logger
	// Process
	*lib.Process
}

const (
	// sendBackoffBase is an initial (minimum) backoff value.
	sendBackoffBase = 1 * time.Second
	// sendBackoffMax is a backoff threshold
	sendBackoffMax = 10 * time.Second
	// sendBackoffNumTries is the maximum number of backoff tries
	sendBackoffNumTries = 5
)

// NewApp creates new app instance
func NewApp(c *StartCmdConfig, log *slog.Logger) (*App, error) {
	app := &App{Config: c, log: log}

	app.eventsJob = NewEventsJob(app)
	app.sessionEventsJob = NewSessionEventsJob(app)

	return app, nil
}

// Run initializes and runs a watcher and a callback server
func (a *App) Run(ctx context.Context) error {
	a.Process = lib.NewProcess(ctx)

	err := a.init(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	a.SpawnCriticalJob(a.eventsJob)
	a.SpawnCriticalJob(a.sessionEventsJob)
	a.SpawnCritical(a.sessionEventsJob.processMissingRecordings)
	<-a.Process.Done()

	return a.Err()
}

// Err returns the error app finished with.
func (a *App) Err() error {
	return trace.NewAggregate(a.eventsJob.Err(), a.sessionEventsJob.Err())
}

// WaitReady waits for http and watcher service to start up.
func (a *App) WaitReady(ctx context.Context) (bool, error) {
	mainReady, err := a.eventsJob.WaitReady(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}

	sessionConsumerReady, err := a.sessionEventsJob.WaitReady(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}

	return mainReady && sessionConsumerReady, nil
}

// SendEvent sends an event to fluentd. Shared method used by jobs.
func (a *App) SendEvent(ctx context.Context, url string, e *TeleportEvent) error {
	if !a.Config.DryRun {
		backoff := backoff.NewDecorr(sendBackoffBase, sendBackoffMax, clockwork.NewRealClock())
		backoffCount := sendBackoffNumTries

		for {
			err := a.Fluentd.Send(ctx, url, e.Event)
			if err == nil {
				break
			}

			a.log.DebugContext(ctx, "Error sending event to fluentd", "error", err)

			bErr := backoff.Do(ctx)
			if bErr != nil {
				return trace.Wrap(err)
			}

			backoffCount--
			if backoffCount < 0 {
				if lib.IsCanceled(err) {
					return nil
				}
				a.log.ErrorContext(
					ctx,
					"Failed to send event to fluentd",
					"error", err,
					"attempts", sendBackoffNumTries,
				)
				return trace.Wrap(err)
			}
		}
	}

	fields := []slog.Attr{
		slog.String("id", e.ID),
		slog.String("type", e.Type),
		slog.Time("ts", e.Time),
		slog.Int64("index", e.Index),
	}
	if e.SessionID != "" {
		fields = append(fields, slog.String("sid", e.SessionID))
	}
	a.log.LogAttrs(
		ctx, slog.LevelDebug, "Event sent",
		fields...,
	)

	return nil
}

// init initializes application state
func (a *App) init(ctx context.Context) error {
	a.Config.Dump(ctx, a.log)

	var err error
	a.client, err = newClient(ctx, a.log, a.Config)
	if err != nil {
		return trace.Wrap(err)
	}

	a.State, err = NewState(a.Config, a.log)
	if err != nil {
		return trace.Wrap(err)
	}

	err = a.setStartTime(ctx, a.State)
	if err != nil {
		return trace.Wrap(err)
	}

	a.Fluentd, err = NewFluentdClient(&a.Config.FluentdConfig, a.log)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// setStartTime sets start time or fails if start time has changed from the last run
func (a *App) setStartTime(ctx context.Context, s *State) error {
	prevStartTime, err := s.GetStartTime()
	if err != nil {
		return trace.Wrap(err)
	}

	if prevStartTime == nil {
		a.log.DebugContext(ctx, "Setting start time", "value", a.Config.StartTime)

		t := a.Config.StartTime
		if t == nil {
			now := time.Now().UTC().Truncate(time.Second)
			t = &now
		}

		return s.SetStartTime(t)
	}

	// If there is a time saved in the state and this time does not equal to the time passed from CLI and a
	// time was explicitly passed from CLI
	if prevStartTime != nil && a.Config.StartTime != nil && *prevStartTime != *a.Config.StartTime {
		return trace.Errorf("You can not change start time in the middle of ingestion. To restart the ingestion, rm -rf %v", a.Config.StorageDir)
	}

	return nil
}

// RegisterSession registers new session
func (a *App) RegisterSession(ctx context.Context, e *TeleportEvent) {
	if err := a.sessionEventsJob.RegisterSession(ctx, e); err != nil {
		a.log.ErrorContext(ctx, "Registering session", "error", err)
	}
}

func (a *App) Profile() {
	if err := diagnostics.Profile(filepath.Join(a.Config.StorageDir, "profiles")); err != nil {
		a.log.WarnContext(context.TODO(), "Failed to capture profiles", "error", err)
	}
}
