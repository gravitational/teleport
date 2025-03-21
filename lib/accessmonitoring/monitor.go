/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

package accessmonitoring

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/semaphore"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
)

const (
	// componentName specifies the access monitor component name used for debugging.
	componentName = "access_monitor"

	// maxAccessRequestConcurrency specifies the maximum number of concurretly
	// handled access request events.
	maxAccessRequestConcurrency = 20
	// maxAccessRequestDeadline specifies the maximum time allowed to handle an
	// access request event.
	maxAccessRequestDeadline = 10 * time.Second
)

// EventHandler describes a function that can handle an access event.
type EventHandler func(ctx context.Context, event types.Event) error

// Config specifies the access monitor configuration.
type Config struct {
	// Logger is the logger for the monitor.
	Logger *slog.Logger

	// Backend should be a backend.Backend which can be used for obtaining the
	// lock required to run the service.
	Backend backend.Backend

	// Events is the event monitor. This interface allows us to monitor access
	// events.
	Events types.Events
}

// CheckAndSetDefaults checks and sets default config values.
func (c *Config) CheckAndSetDefaults() error {
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	if c.Backend == nil {
		return trace.BadParameter("backend: must be non-nil")
	}
	if c.Events == nil {
		return trace.BadParameter("events: must be non-nil")
	}
	return nil
}

// AccessMonitor is an access monitoring service that monitors access events,
// then executes the configured set of handlers for each event.
type AccessMonitor struct {
	cfg Config

	// ruleHandlers contains the list of access monitoring rule event handlers.
	ruleHandlers []EventHandler
	// requestHandlers contains the list access request event handlers.
	requestHandlers []EventHandler
}

// NewAccessMonitor returns a new access monitor.
func NewAccessMonitor(cfg Config) (*AccessMonitor, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &AccessMonitor{
		cfg: cfg,
	}, nil
}

// AddAccessMonitoringRuleHandler appends the access monitoring rule event handler.
// These handlers are executed whenever a new access monitoring rule event is
// observed.
func (s *AccessMonitor) AddAccessMonitoringRuleHandler(handler EventHandler) {
	s.ruleHandlers = append(s.ruleHandlers, handler)
}

// AddAccessRequestHandler appends the access request handler.
// These handlers are executed whenever a new access request event is observed.
func (s *AccessMonitor) AddAccessRequestHandler(handler EventHandler) {
	s.requestHandlers = append(s.requestHandlers, handler)
}

// Run runs the access monitor. Restarts on failure.
func (s *AccessMonitor) Run(ctx context.Context) error {
	const retryJitter = 10 * time.Second
	const retryInterval = 30 * time.Second
	const lockTTL = time.Minute

	waitWithJitter := retryutils.SeventhJitter(retryJitter)
	for {
		err := backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
			LockConfiguration: backend.LockConfiguration{
				Backend:            s.cfg.Backend,
				LockNameComponents: []string{componentName},
				TTL:                lockTTL,
				RetryInterval:      retryInterval,
			},
		}, s.run)
		if err != nil {
			s.cfg.Logger.ErrorContext(
				ctx,
				"Encountered a fatal error, it will restart after backoff.",
				"component", componentName,
				"error", err,
				"restart_after", waitWithJitter,
			)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(waitWithJitter):
		}
	}
}

func (s *AccessMonitor) run(ctx context.Context) error {
	// Initialize the watcher.
	const allowPartialSuccessFalse = false
	watcher, err := s.cfg.Events.NewWatcher(ctx, types.Watch{
		Name: componentName,
		Kinds: []types.WatchKind{
			{Kind: types.KindAccessRequest},
			{Kind: types.KindAccessMonitoringRule},
		},
		AllowPartialSuccess: allowPartialSuccessFalse,
	})
	if err != nil {
		return trace.Wrap(err, "failed to create watcher")
	}
	defer func() {
		if err := watcher.Close(); err != nil {
			s.cfg.Logger.ErrorContext(ctx, "Failed to close watcher.", "error", err)
		}
	}()

	// Watcher is open, expect init event.
	// Not inspecting WatchStatus because ParitalSuccess is not allowed.
	select {
	case initEvent := <-watcher.Events():
		if initEvent.Type != types.OpInit {
			return trace.BadParameter("watcher yielded %[1]v (%[1]d) as first event, expected Init (this is a bug)", initEvent.Type)
		}
		// Initialize the access monitoring rule handler caches.
		if err := handleEvent(ctx, s.ruleHandlers, initEvent); err != nil {
			return trace.Wrap(err, "failed to initialize access monitoring rule handler")
		}
	case <-watcher.Done():
		return trace.Wrap(watcher.Error())
	case <-ctx.Done():
		return ctx.Err()
	}

	// Limit the number of concurrently handled access request events.
	lock := semaphore.NewWeighted(maxAccessRequestConcurrency)

	for {
		select {
		case event := <-watcher.Events():
			switch event.Resource.GetKind() {

			// Handle access monitoring rule events.
			case types.KindAccessMonitoringRule:
				if err := handleEvent(ctx, s.ruleHandlers, event); err != nil {
					s.cfg.Logger.ErrorContext(ctx,
						"Failed to handle access monitoring rule event.",
						"error", err,
						"event", event.String())
				}

			// Handle access request events.
			case types.KindAccessRequest:
				if err := lock.Acquire(ctx, 1); err != nil {
					return trace.Wrap(err, "failed to acquire access request semaphore")
				}
				go func() {
					defer lock.Release(1)
					eventCtx, cancel := context.WithTimeout(ctx, maxAccessRequestDeadline)
					defer cancel()
					if err := handleEvent(eventCtx, s.requestHandlers, event); err != nil {
						s.cfg.Logger.ErrorContext(ctx,
							"Failed to handle access request event.",
							"error", err,
							"event", event.String())
					}
				}()
			}

		case <-watcher.Done():
			err := watcher.Error()
			switch {
			case errors.Is(err, context.Canceled):
				return nil
			case err != nil:
				return trace.Wrap(err, "watcher failed")
			default:
				return trace.BadParameter("watcher closed unexpectedly")
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// handleEvent delegates the event to the event handlers.
func handleEvent(ctx context.Context, handlers []EventHandler, event types.Event) error {
	var errors []error
	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}
