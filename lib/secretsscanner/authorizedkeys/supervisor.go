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
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/utils/retryutils"
)

var errShutdown = errors.New("watcher is shutting down")

type supervisorRunnerConfig struct {
	clock                 clockwork.Clock
	tickerInterval        time.Duration
	runner                func(context.Context) error
	checkIfMonitorEnabled func(context.Context) (bool, error)
	logger                *slog.Logger
}

// supervisorRunner runs the runner based on the checkIfMonitorEnabled result.
// If the monitor is enabled, the runner is started. If the monitor is disabled,
// the runner is stopped if it is running.
// The checkIfMonitorEnabled is evaluated every tickerInterval duration to determine
// if the monitor should be started or stopped.
// tickerInterval is jittered to prevent all watchers from running at the same time.
// If the watcher is stopped, it will be restarted after the next checkIfMonitorEnabled evaluation.
func supervisorRunner(parentCtx context.Context, cfg supervisorRunnerConfig) error {
	var (
		isRunning    = false
		runCtx       context.Context
		runCtxCancel context.CancelCauseFunc
		wg           sync.WaitGroup
		mu           sync.Mutex
	)

	getIsRunning := func() bool {
		mu.Lock()
		defer mu.Unlock()
		return isRunning
	}

	setIsRunning := func(s bool) {
		mu.Lock()
		defer mu.Unlock()
		isRunning = s
	}

	runRoutine := func(ctx context.Context, cancel context.CancelCauseFunc) {
		defer func() {
			wg.Done()
			cancel(errShutdown)
			setIsRunning(false)
		}()
		if err := cfg.runner(ctx); err != nil && !errors.Is(err, errShutdown) {
			cfg.logger.WarnContext(ctx, "Runner failed", "error", err)
		}
	}

	jitterFunc := retryutils.HalfJitter
	t := cfg.clock.NewTimer(jitterFunc(cfg.tickerInterval))
	for {
		switch enabled, err := cfg.checkIfMonitorEnabled(parentCtx); {
		case err != nil:
			cfg.logger.WarnContext(parentCtx, "Failed to check if authorized keys report is enabled", "error", err)
		case enabled && !getIsRunning():
			runCtx, runCtxCancel = context.WithCancelCause(parentCtx)
			setIsRunning(true)
			wg.Add(1)
			go runRoutine(runCtx, runCtxCancel)
		case !enabled && getIsRunning():
			runCtxCancel(errShutdown)
			// Wait for the runner to stop before checking if the monitor is enabled again.
			wg.Wait()
		}

		select {
		case <-t.Chan():
			if !t.Stop() {
				select {
				case <-t.Chan():
				default:
				}
			}
			t.Reset(jitterFunc(cfg.tickerInterval))
		case <-parentCtx.Done():
			return nil
		}
	}
}
