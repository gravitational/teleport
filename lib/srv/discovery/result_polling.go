/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package discovery

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/lib/cloud/azure"
)

const (
	// azureResultPollingFrequency is how often Azure install results are polled.
	azureResultPollingFrequency = 30 * time.Second
	// azureResultPollingParallelism is the upper limit of the number of
	// goroutines that poll Azure install command results.
	azureResultPollingParallelism = 100
)

var (
	// azureResultPollingTimeout is the amount of time given to collect all
	// pending Azure install results.
	azureResultPollingTimeout = max(
		azure.GetRunCommandTimeout()+(5*time.Minute),
		10*time.Minute,
	)
)

// poller represents an operation that can be polled.
type poller interface {
	// Poll polls for a result. It returns true when polling is complete, either
	// because the result is ready or the poller reached a terminal failure.
	Poll(ctx context.Context) bool
}

// newPollerManager makes a new [pollerManager].
func newPollerManager(log *slog.Logger, clock clockwork.Clock, pollers ...poller) *pollerManager {
	return &pollerManager{
		log:     log,
		clock:   clock,
		pending: pollers,
	}
}

// pollerManager manages a queue of pollers, polling each one in parallel
// batches.
type pollerManager struct {
	log   *slog.Logger
	clock clockwork.Clock

	// pending is the list of results to poll next.
	pending []poller
	// doneCount is the count of pollers that completed.
	doneCount int
}

// runWithTimeout calls run with a timeout.
func (m *pollerManager) runWithTimeout(ctx context.Context, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	m.run(ctx)
}

// run polls all pending pollers until either all of the results have been
// resolved or the context is canceled.
func (m *pollerManager) run(ctx context.Context) {
	ticker := m.clock.NewTicker(azureResultPollingFrequency)
	defer ticker.Stop()

	for m.pollOnce(ctx) {
		select {
		case <-ctx.Done():
			m.log.DebugContext(ctx, "Aborted installation result collection",
				"error", ctx.Err(),
				"results", m.resultsLogValue(),
			)
			return
		case <-ticker.Chan():
		}
	}
	m.log.DebugContext(ctx, "Successfully completed installation result collection",
		"results", m.resultsLogValue(),
	)
}

// pollOnce iterates over all enqueued items, polls each item once for a result,
// and returns true if there are items remaining its queue.
func (m *pollerManager) pollOnce(ctx context.Context) bool {
	if len(m.pending) == 0 {
		return false
	}

	m.log.DebugContext(ctx, "Polling pending installation results",
		"results", m.resultsLogValue(),
	)
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(azureResultPollingParallelism)

	pollers := m.pending
	m.pending = nil
	var mu sync.Mutex
	for _, poller := range pollers {
		g.Go(func() error {
			done := poller.Poll(ctx)
			mu.Lock()
			defer mu.Unlock()
			if !done {
				m.pending = append(m.pending, poller)
				return nil
			}
			m.doneCount++
			return nil
		})
	}
	_ = g.Wait()
	return len(m.pending) > 0
}

// resultsLogValue returns a [slog.Value] summarizing the poller results.
// The caller of this func must hold the poller state mutex.
func (m *pollerManager) resultsLogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("done", m.doneCount),
		slog.Int("pending", len(m.pending)),
		slog.Int("total", m.doneCount+len(m.pending)),
	)
}
