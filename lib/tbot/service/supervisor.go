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
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/tbot/service/status"
)

// Supervisor is responsible for managing the lifecycle of tbot's component
// services.
type Supervisor struct {
	logger *slog.Logger
	clock  clockwork.Clock

	mu       sync.Mutex
	services map[string]InternalService
	started  bool
}

// SupervisorConfig contains the configuration options for Supervisor.
type SupervisorConfig struct {
	// Logger used to log messages and errors.
	Logger *slog.Logger

	// Clock allows you to override the clock in unit tests.
	Clock clockwork.Clock
}

func (cfg *SupervisorConfig) checkAndSetDefaults() error {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
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
		clock:    cfg.Clock,
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

	s.services[service.getName()] = service
	return nil
}

// Run the registered services until the given context is canceled or a service
// exits with an irrecoverable error.
func (s *Supervisor) Run(ctx context.Context) error {
	group, groupCtx := errgroup.WithContext(ctx)

	s.mu.Lock()
	for _, svc := range s.services {
		svc := svc
		group.Go(func() error {
			return s.superviseService(groupCtx, svc)
		})
	}
	s.mu.Unlock()

	err := group.Wait()
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// OneShot runs any registered services that implement OneShotHandler.
func (s *Supervisor) OneShot(ctx context.Context) error {
	ctx = withOneShot(ctx)

	// Note: we don't use errgroup here because we don't want one service's
	// error to immediately cancel other services, in case they can gracefully
	// degrade instead.
	var wg sync.WaitGroup

	s.mu.Lock()
	errCh := make(chan error, len(s.services))
	for _, svc := range s.services {
		wg.Add(1)

		svc := svc
		go func() {
			errCh <- s.oneShotService(ctx, svc)
			wg.Done()
		}()
	}
	s.mu.Unlock()

	wg.Wait()
	close(errCh)

	return trace.NewAggregateFromChannel(errCh, ctx)
}

func (s *Supervisor) superviseService(ctx context.Context, svc InternalService) error {
	logger := s.logger.With("service", svc.getName())

	setStatus := func(status status.Status) {
		if svc.setStatus(status) {
			logger.DebugContext(ctx, "Service status changed", "new_status", status)
		}
	}

	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		Driver: retryutils.NewExponentialDriver(1 * time.Second),
		Jitter: retryutils.HalfJitter,
		Max:    1 * time.Minute,
		Clock:  s.clock,
	})
	if err != nil {
		return trace.Wrap(err, "creating retrier")
	}

	for {
		err := svc.runHandler(ctx, &Runtime{setStatusFn: setStatus})
		setStatus(status.Failed)

		if IsIrrecoverableError(err) {
			logger.ErrorContext(ctx, "Service encountered irrecoverable error; supervisor is shutting down", "error", err)
			return err
		} else {
			logger.InfoContext(ctx, "Service exited", "error", err)
		}

		retry.Inc()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-retry.After():
			logger.DebugContext(ctx, "Retrying failed service")
		}
	}
}

func (s *Supervisor) oneShotService(ctx context.Context, svc InternalService) error {
	logger := s.logger.With("service", svc.getName())

	setStatus := func(status status.Status) {
		if svc.setStatus(status) {
			logger.DebugContext(ctx, "Service status changed", "new_status", status)
		}
	}

	err := svc.runOneShotHandler(ctx)
	switch {
	case errors.Is(err, errNoOneShotHandler):
		logger.DebugContext(ctx, "Service does not support one-shot mode")
		return nil
	case err != nil:
		logger.ErrorContext(ctx, "Service returned error", "error", err)
		setStatus(status.Failed)
	default:
		setStatus(status.Ready)
	}

	return err
}

// InternalService is the internal API surface of Service[HandlerT] used by the
// supervisor. It uses the non-type-parameterized methods.
type InternalService interface {
	getName() string
	runHandler(context.Context, *Runtime) error
	runOneShotHandler(ctx context.Context) error
	setStatus(status.Status) bool
}

// ctxKeyOneShot is used to mark the context as running under a supervisor in
// one-shot mode, so that we can unblock earlier in Service.WaitForStatus.
type ctxKeyOneShot struct{}

func withOneShot(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeyOneShot{}, struct{}{})
}

func isOneShot(ctx context.Context) bool {
	return ctx.Value(ctxKeyOneShot{}) != nil
}
