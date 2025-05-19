/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/lib/tbot/service/status"
)

// Supervisor is responsible for managing the lifecycle of tbot's component
// services.
type Supervisor struct {
	logger *slog.Logger

	mu       sync.Mutex
	services map[string]InternalService
	started  bool
}

// SupervisorConfig contains the configuration options for Supervisor.
type SupervisorConfig struct {
	// Logger used to log messages and errors.
	Logger *slog.Logger
}

func (cfg *SupervisorConfig) checkAndSetDefaults() error {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return nil
}

// NewSupervisor creates a new supervisor.
func NewSupervisor(cfg SupervisorConfig) (*Supervisor, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, err
	}
	return &Supervisor{
		services: make(map[string]InternalService),
		logger:   cfg.Logger,
	}, nil
}

// Register the given service. Returns an error if the service has already been
// registered, or if the supervisor is already running.
//
// TODO: support registering services dynamically at-runtime.
func (s *Supervisor) Register(service InternalService) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return trace.Errorf("cannot register services once the supervisor has started")
	}

	if _, ok := s.services[service.getName()]; ok {
		return trace.Errorf("service %q is already registered", service.getName())
	}

	if !service.registerToSupervisor() {
		return trace.Errorf("cannot register a service to more than one supervisor")
	}

	s.services[service.getName()] = service
	return nil
}

// Run the registered services until the given context is canceled or a service
// exits. If the given context is canceled, its error will be returned.
func (s *Supervisor) Run(ctx context.Context) error {
	group, groupCtx := errgroup.WithContext(ctx)

	s.mu.Lock()

	if s.started {
		s.mu.Unlock()
		return trace.Errorf("cannot run a supervisor more than once")
	}
	s.started = true

	for _, svc := range s.services {
		svc := svc
		group.Go(func() error {
			return trace.Wrap(
				s.runService(groupCtx, svc, false),
				"service: %s", svc.getName(),
			)
		})
	}
	s.mu.Unlock()

	err := group.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// OneShot runs any registered services that implement OneShotHandler.
func (s *Supervisor) OneShot(ctx context.Context) error {
	// Note: we don't use errgroup here because we don't want one service's
	// error to immediately cancel other services, in case they can gracefully
	// degrade instead.
	var wg sync.WaitGroup

	s.mu.Lock()

	if s.started {
		s.mu.Unlock()
		return trace.Errorf("cannot run a supervisor more than once")
	}
	s.started = true

	errCh := make(chan error, len(s.services))
	for _, svc := range s.services {
		wg.Add(1)

		svc := svc
		go func() {
			err := s.runService(ctx, svc, true)
			errCh <- trace.Wrap(err, "service: %s", svc.getName())
			wg.Done()
		}()
	}
	s.mu.Unlock()

	wg.Wait()
	close(errCh)

	return trace.NewAggregateFromChannel(errCh, ctx)
}

func (s *Supervisor) runService(ctx context.Context, svc InternalService, oneShot bool) error {
	defer svc.finalize()

	logger := s.logger.With("service", svc.getName())

	setStatus := func(status status.Status) {
		if svc.setStatus(status) {
			logger.DebugContext(ctx, "Service status changed", "new_status", status)
		}
	}

	var err error
	if oneShot {
		err = svc.runOneShotHandler(ctx)
	} else {
		err = svc.runHandler(ctx, &Runtime{setStatusFn: setStatus})
	}

	switch {
	case ctx.Err() != nil:
		// Handler exited because the context was cancelled (possibly because
		// another service failed).
		setStatus(status.Failed)
		return nil
	case errors.Is(err, errNoOneShotHandler):
		// Supervisor running in one-shot mode but handler doesn't support it.
		logger.DebugContext(ctx, "Service does not support one-shot mode")
		return nil
	case err != nil:
		// Handler returned an error.
		logger.ErrorContext(ctx, "Service returned error", "error", err)
		setStatus(status.Failed)
		return err
	case oneShot:
		// One-shot handler succeeded.
		setStatus(status.Ready)
		return nil
	default:
		// Long-running handler returned early without the context being canceled.
		setStatus(status.Failed)
		logger.ErrorContext(ctx, "Service exited unexpectedly without error")
		return trace.Errorf("service named %q exited unexpectedly without error", svc.getName())
	}
}

// InternalService is the internal API surface of Service[HandlerT] used by the
// supervisor. It uses the non-type-parameterized methods.
type InternalService interface {
	getName() string
	runHandler(context.Context, *Runtime) error
	runOneShotHandler(ctx context.Context) error
	setStatus(status.Status) bool
	registerToSupervisor() bool
	finalize()
}
