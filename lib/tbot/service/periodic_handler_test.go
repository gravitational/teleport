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

func TestPeriodicHandler(t *testing.T) {
	// Timer that controls when the periodic handler runs.
	timer := newFakeTimer()

	calls := make(chan chan error, 1)
	handler, err := service.NewPeriodicHandler(
		oneShotHandlerFunc(func(ctx context.Context) error {
			result := make(chan error, 1)
			select {
			case calls <- result:
			case <-ctx.Done():
				return ctx.Err()
			}

			select {
			case err := <-result:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		}),
		service.PeriodicHandlerOptions{
			Interval:    5 * time.Minute,
			Clock:       fakeClock{timer: timer},
			MaxAttempts: 2,
			Logger:      utils.NewSlogLoggerForTests(),
		},
	)
	require.NoError(t, err)
	svc := service.NewService("periodic-service", handler)

	supervisor, err := service.NewSupervisor(service.SupervisorConfig{
		Logger: utils.NewSlogLoggerForTests(),
	})
	require.NoError(t, err)
	require.NoError(t, supervisor.Register(svc))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = supervisor.Run(ctx) }()

	select {
	case result := <-calls:
		result <- errors.New("KABOOM")
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for initial call")
	}

	select {
	case result := <-calls:
		result <- errors.New("KABOOM")
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for retry")
	}

	timer.WaitForCaller(t)

	// Reaching MaxRetries should've marked the service as failed.
	require.Equal(t, status.Failed.String(), svc.Status().String())

	select {
	case result := <-calls:
		result <- nil
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for next call")
	}

	timer.WaitForCaller(t)

	// Successful retry should've marked the service as ready.
	require.Equal(t, status.Ready.String(), svc.Status().String())
}

type fakeClock struct {
	clockwork.Clock

	timer clockwork.Timer
}

func (fakeClock) After(time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	ch <- time.Now()
	return ch
}

func (f fakeClock) NewTimer(time.Duration) clockwork.Timer {
	return f.timer
}

func newFakeTimer() *fakeTimer {
	return &fakeTimer{
		callback: make(chan chan time.Time, 1),
	}
}

type fakeTimer struct {
	callback chan chan time.Time
}

func (f *fakeTimer) WaitForCaller(t *testing.T) {
	t.Helper()

	select {
	case cb := <-f.callback:
		cb <- time.Now()
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for call to Chan")
	}
}

func (f *fakeTimer) Chan() <-chan time.Time {
	timeCh := make(chan time.Time, 1)
	f.callback <- timeCh
	return timeCh
}

func (*fakeTimer) Reset(time.Duration) bool { return true }
func (*fakeTimer) Stop() bool               { return true }
