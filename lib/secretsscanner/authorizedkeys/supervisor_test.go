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
	"testing/synctest"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestSupervisorRunner(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		clock := clockwork.NewRealClock()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		var mu sync.Mutex
		var running bool
		readRunning := func() bool {
			mu.Lock()
			defer mu.Unlock()
			return running
		}
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

		// initially enabled
		synctest.Wait()

		require.True(t, readRunning(), "expected runner to be running, but it is not")

		// disable runner
		disable()

		time.Sleep(2 * time.Second)
		synctest.Wait()

		require.False(t, readRunning(), "expected runner to be stopped, but it is running")

		// re-enable runner
		enable()

		time.Sleep(2 * time.Second)
		synctest.Wait()

		require.True(t, readRunning(), "expected runner to be running, but it is not")

		// disable runner again
		disable()

		time.Sleep(2 * time.Second)
		synctest.Wait()

		require.False(t, readRunning(), "expected runner to be stopped, but it is running")

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
