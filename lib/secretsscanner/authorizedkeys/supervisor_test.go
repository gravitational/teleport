/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package authorizedkeys

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestSupervisorRunner(t *testing.T) {
	// Create a mock clock
	clock := clockwork.NewFakeClock()

	t.Run("runner starts and stops based on monitor state", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var mu sync.Mutex
		var running bool

		runner := func(ctx context.Context) error {
			mu.Lock()
			running = true
			mu.Unlock()
			<-ctx.Done()
			mu.Lock()
			running = false
			mu.Unlock()
			return nil
		}

		checker, enable, disable := checkIfMonitorEnabled()
		enable()

		cfg := supervisorRunnerConfig{
			clock:                 clock,
			tickerInterval:        1 * time.Second,
			runner:                runner,
			checkIfMonitorEnabled: checker,
			logger:                slog.Default(),
		}

		g, ctx := errgroup.WithContext(ctx)
		g.Go(func() error {
			return supervisorRunner(ctx, cfg)
		})

		require.Eventually(t, func() bool {
			mu.Lock()
			defer mu.Unlock()
			return running
		}, 100*time.Millisecond, 10*time.Millisecond, "expected runner to start, but it did not")

		disable()

		clock.BlockUntil(1)
		clock.Advance(2 * time.Second)

		require.Eventually(t, func() bool {
			mu.Lock()
			defer mu.Unlock()
			return !running
		}, 100*time.Millisecond, 10*time.Millisecond, "expected runner to stop, but it did not")

		enable()
		clock.BlockUntil(1)
		clock.Advance(2 * time.Second)

		require.Eventually(t, func() bool {
			mu.Lock()
			defer mu.Unlock()
			return running
		}, 100*time.Millisecond, 10*time.Millisecond, "expected runner to re-start, but it did not")

		disable()
		clock.BlockUntil(1)
		clock.Advance(2 * time.Second)

		require.Eventually(t, func() bool {
			mu.Lock()
			defer mu.Unlock()
			return !running
		}, 100*time.Millisecond, 10*time.Millisecond, "expected runner to re-stop, but it did not")

		// Cancel the context to stop the supervisor
		cancel()
		if err := g.Wait(); err != nil {
			t.Fatal(err)
		}
	})

}

func checkIfMonitorEnabled() (checker func(context.Context) (bool, error), enable func(), disable func()) {
	var (
		enabled bool
		mu      sync.Mutex
	)
	return func(ctx context.Context) (bool, error) {
			mu.Lock()
			defer mu.Unlock()
			return enabled, nil
		}, func() {
			mu.Lock()
			defer mu.Unlock()
			enabled = true
		}, func() {
			mu.Lock()
			defer mu.Unlock()
			enabled = false
		}
}
