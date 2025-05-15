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
package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/service"
	"github.com/gravitational/teleport/lib/tbot/service/status"
	"github.com/gravitational/teleport/lib/utils"
)

func TestService_EndToEnd(t *testing.T) {
	t.Parallel()

	a := service.NewService(
		"service-a",
		handlerFunc(func(ctx context.Context, runtime *service.Runtime) error {
			runtime.SetStatus(status.Failed)
			runtime.SetStatus(status.Ready)
			<-ctx.Done()
			return nil
		}),
	)

	b := service.NewService(
		"service-b",
		handlerFunc(func(ctx context.Context, runtime *service.Runtime) error {
			if _, err := a.Wait(ctx); err != nil {
				return errors.New("service a isn't ready")
			}
			runtime.SetStatus(status.Ready)
			<-ctx.Done()
			return nil
		}),
	)

	supervisor, err := service.NewSupervisor(service.SupervisorConfig{
		Logger: utils.NewSlogLoggerForTests(),
	})
	require.NoError(t, err)
	require.NoError(t, supervisor.Register(a))
	require.NoError(t, supervisor.Register(b))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	errCh := make(chan error)
	go func() { errCh <- supervisor.Run(ctx) }()

	_, err = b.Wait(ctx)
	require.NoError(t, err)
	require.Equal(t, status.Ready.String(), b.Status().String())

	cancel()
	require.NoError(t, <-errCh)
}

func TestService_OneShot(t *testing.T) {
	a := service.NewService(
		"service-a",
		oneShotHandlerFunc(func(context.Context) error {
			return nil
		}),
	)

	b := service.NewService(
		"service-b",
		oneShotHandlerFunc(func(ctx context.Context) error {
			return errors.New("uh-oh")
		}),
	)

	c := service.NewService(
		"service-c",
		oneShotHandlerFunc(func(ctx context.Context) error {
			// This should fail as soon a service-b returns its error, rather
			// than blocking indefinitely.
			_, err := b.Wait(ctx)
			return err
		}),
	)

	supervisor, err := service.NewSupervisor(service.SupervisorConfig{
		Logger: utils.NewSlogLoggerForTests(),
	})
	require.NoError(t, err)
	require.NoError(t, supervisor.Register(a))
	require.NoError(t, supervisor.Register(b))
	require.NoError(t, supervisor.Register(c))

	err = supervisor.OneShot(context.Background())
	require.ErrorContains(t, err, "uh-oh")

	require.Equal(t, status.Ready.String(), a.Status().String())
	require.Equal(t, status.Failed.String(), b.Status().String())
	require.Equal(t, status.Failed.String(), c.Status().String())
}

func TestService_Retries(t *testing.T) {
	called := make(chan struct{}, 1)

	svc := service.NewService(
		"service-a",
		handlerFunc(func(ctx context.Context, _ *service.Runtime) error {
			called <- struct{}{}
			return errors.New("uh-oh")
		}),
	)

	supervisor, err := service.NewSupervisor(service.SupervisorConfig{
		Logger: utils.NewSlogLoggerForTests(),
		Clock:  &retryClock{blockAfter: 2},
	})
	require.NoError(t, err)
	require.NoError(t, supervisor.Register(svc))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	errCh := make(chan error)
	go func() { errCh <- supervisor.Run(ctx) }()

	// Expect the service to be called multiple times.
	for i := 0; i < 2; i++ {
		<-called
	}

	cancel()
	require.NoError(t, <-errCh)
}

func TestService_IrrecoverableError(t *testing.T) {
	var calls int

	a := service.NewService(
		"service-a",
		handlerFunc(func(context.Context, *service.Runtime) error {
			calls++
			return service.IrrecoverableError(errors.New("uh-oh"))
		}),
	)

	b := service.NewService(
		"service-b",
		handlerFunc(func(ctx context.Context, runtime *service.Runtime) error {
			runtime.SetStatus(status.Ready)
			<-ctx.Done()
			return nil
		}),
	)

	supervisor, err := service.NewSupervisor(service.SupervisorConfig{
		Logger: utils.NewSlogLoggerForTests(),
	})
	require.NoError(t, err)
	require.NoError(t, supervisor.Register(a))
	require.NoError(t, supervisor.Register(b))

	err = supervisor.Run(context.Background())
	require.ErrorContains(t, err, "uh-oh")
	require.Equal(t, 1, calls)

	require.Equal(t, status.Failed.String(), a.Status().String())
	require.Equal(t, status.Failed.String(), b.Status().String())
}

func TestService_MultipleSupervisors(t *testing.T) {
	svc := service.NewService(
		"service",
		handlerFunc(nil),
	)

	a, err := service.NewSupervisor(service.SupervisorConfig{})
	require.NoError(t, err)

	b, err := service.NewSupervisor(service.SupervisorConfig{})
	require.NoError(t, err)

	require.NoError(t, a.Register(svc))
	require.ErrorContains(t, b.Register(svc), "cannot register a service to more than one supervisor")
}

func TestService_SupervisorRunMultipleTimes(t *testing.T) {
	sup, err := service.NewSupervisor(service.SupervisorConfig{})
	require.NoError(t, err)
	require.NoError(t, sup.Run(context.Background()))

	require.ErrorContains(t, sup.Run(context.Background()), "cannot run a supervisor more than once")
	require.ErrorContains(t, sup.OneShot(context.Background()), "cannot run a supervisor more than once")
}

type handlerFunc func(context.Context, *service.Runtime) error

func (fn handlerFunc) Run(ctx context.Context, runtime *service.Runtime) error {
	return fn(ctx, runtime)
}

type oneShotHandlerFunc func(ctx context.Context) error

func (oneShotHandlerFunc) Run(context.Context, *service.Runtime) error {
	return errors.New("not a long-running service")
}

func (fn oneShotHandlerFunc) OneShot(ctx context.Context) error {
	return fn(ctx)
}

type retryClock struct {
	clockwork.Clock

	blockAfter int
	calls      int
}

func (c *retryClock) After(time.Duration) <-chan time.Time {
	c.calls++

	ch := make(chan time.Time, 1)
	if c.calls <= c.blockAfter {
		ch <- time.Now()
	}
	return ch
}
