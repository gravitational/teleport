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
	"slices"

	"github.com/gravitational/teleport/lib/tbot/service/status"
)

// Handler implements the logic of a long-running service.
type Handler interface {
	// Run performs the task (potentially on an interval) until the given
	// context is canceled, or an irrecoverable error is encountered.
	//
	// It should call runtime.SetStatus to report its health to the supervisor
	// and other services. If Run returns an error, the service's status will be
	// set to Failed and the supervisor will restart the handler after some
	// backoff and jitter.
	//
	// If the handler encounters an error that cannot possibly be resolved, it
	// should return the error wrapped with IrrecoverableError and the supervisor
	// will shut down all of the other services.
	Run(ctx context.Context, runtime *Runtime) error
}

// OneShotHandler can be implemented by a service to support running in one-shot
// mode rather than as a long-running daemon.
type OneShotHandler interface {
	// OneShot performs the task once and then exits. If it returns nil, the
	// service's status will be set to Ready otherwise it will be set to Failed.
	OneShot(ctx context.Context) error
}

// NewService creates a service with the given name and handler.
func NewService[HandlerT Handler](name string, handler HandlerT) *Service[HandlerT] {
	return &Service[HandlerT]{
		name:    name,
		handler: handler,
		status:  NewWatchedValue(status.Initializing),
	}
}

// Service implements a user-configurable part of tbot, such as generating AWS
// credentials, serving the SPIFFE Workload API, or proxying database traffic.
//
// Services have a status that describes whether they're currently healthy.
// Dependent services can subscribe to status changes using WatchStatusChanges
// or wait for the service to have a given status using WaitForStatus.
//
// If the service's handler implements OneShotHandler, it can be used with the
// `--one-shot` flag.
//
// The service lifecycle is managed by a Supervisor.
type Service[HandlerT Handler] struct {
	name    string
	handler HandlerT
	status  *WatchedValue[status.Status]
}

// Status returns the service's current status.
func (s *Service[HandlerT]) Status() status.Status {
	return s.status.Get()
}

// ErrWrongStatus is returned from Service.Wait and Service.WaitForStatus when
// the service is not in the required status.
var ErrWrongStatus = errors.New("supervisor: service has wrong status")

// Wait blocks until the service's status is Ready and then returns the handler.
//
// If the given context is canceled or reaches its deadline, Wait will unblock
// immediately and return the context's error. This is useful for implementing
// timeout/fallback behavior.
//
// When called from another service running in one-shot mode, Wait will block
// until the service's status changes *for the first time* only (because in
// one-shot mode an unhealthy service wouldn't ever recover). If the service
// isn't ready, ErrWrongStatus will be returned.
func (s *Service[HandlerT]) Wait(ctx context.Context) (HandlerT, error) {
	return s.WaitForStatus(ctx, status.Ready)
}

// WaitForStatus blocks until the service's status matches one of the given
// statuses and then returns the handler.
//
// If the given context is canceled or reaches its deadline, WaitForStatus will
// unblock immediately and return the context's error. This is useful for
// implementing timeout/fallback behavior.
//
// When called from another service running in one-shot mode, WaitForStatus will
// block until the service's status changes *for the first time* only (because
// in one-shot mode an unhealthy service wouldn't ever recover). If the service's
// status doesn't match one of the expected statuses, ErrWrongStatus will be returned.
func (s *Service[HandlerT]) WaitForStatus(ctx context.Context, statuses ...status.Status) (HandlerT, error) {
	current, watcher := s.status.Watch()
	defer watcher.Close()

	if slices.Contains(statuses, current) {
		return s.handler, nil
	}

	var zero HandlerT
	if isOneShot(ctx) && current != status.Initializing {
		return zero, ErrWrongStatus
	}

	for {
		current, ok := watcher.Wait(ctx)
		if !ok {
			return zero, ctx.Err()
		}
		if slices.Contains(statuses, current) {
			return s.handler, nil
		}
		if isOneShot(ctx) {
			return zero, ErrWrongStatus
		}
	}
}

// These methods implement the InternalService API used by the supervisor.
func (s *Service[HandlerT]) getName() string                     { return s.name }
func (s *Service[HandlerT]) setStatus(status status.Status) bool { return s.status.Set(status) }
func (s *Service[HandlerT]) runHandler(ctx context.Context, runtime *Runtime) error {
	return s.handler.Run(ctx, runtime)
}
func (s *Service[HandlerT]) runOneShotHandler(ctx context.Context) error {
	var handler any = s.handler
	if os, ok := handler.(OneShotHandler); ok {
		return os.OneShot(ctx)
	}
	return errNoOneShotHandler
}

// errNoOneShotHandler is a sentinel error used to tell the supervisor that the
// service doesn't support one-shot mode.
var errNoOneShotHandler = errors.New("supervisor: handler does not implement OneShotHandler")

// Runtime allows service handlers to communicate with the supervisor and other
// services.
type Runtime struct{ setStatusFn func(status status.Status) }

// SetStatus communicates the service's status with the supervisor and other
// services.
func (rt *Runtime) SetStatus(status status.Status) { rt.setStatusFn(status) }
