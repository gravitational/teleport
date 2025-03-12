/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
)

// Instances contains information about discovered cloud instances from any provider.
type Instances struct {
	EC2   *EC2Instances
	Azure *AzureInstances
	GCP   *GCPInstances
}

// Fetcher fetches instances from a particular cloud provider.
type Fetcher interface {
	// GetInstances gets a list of cloud instances.
	GetInstances(ctx context.Context, rotation bool) ([]Instances, error)
	// GetMatchingInstances finds Instances from the list of nodes
	// that the fetcher matches.
	GetMatchingInstances(nodes []types.Server, rotation bool) ([]Instances, error)
	// GetDiscoveryConfigName returns the DiscoveryConfig name that created this fetcher.
	// Empty for Fetchers created from `teleport.yaml/discovery_service.aws.<Matcher>` matchers.
	GetDiscoveryConfigName() string
	// IntegrationName identifies the integration name whose credentials were used to fetch the resources.
	// Might be empty when the fetcher is using ambient credentials.
	IntegrationName() string
}

// WithTriggerFetchC sets a poll trigger to manual start a resource polling.
func WithTriggerFetchC(triggerFetchC <-chan struct{}) Option {
	return func(w *Watcher) {
		w.triggerFetchC = triggerFetchC
	}
}

// WithPreFetchHookFn sets a function that gets called before each new iteration.
func WithPreFetchHookFn(f func()) Option {
	return func(w *Watcher) {
		w.preFetchHookFn = f
	}
}

// WithClock sets a clock that is used to periodically fetch new resources.
func WithClock(clock clockwork.Clock) Option {
	return func(w *Watcher) {
		w.clock = clock
	}
}

// Watcher allows callers to discover cloud instances matching specified filters.
type Watcher struct {
	// InstancesC can be used to consume newly discovered instances.
	InstancesC     chan Instances
	missedRotation <-chan []types.Server

	fetchersFn     func() []Fetcher
	pollInterval   time.Duration
	clock          clockwork.Clock
	triggerFetchC  <-chan struct{}
	ctx            context.Context
	cancel         context.CancelFunc
	preFetchHookFn func()
}

func (w *Watcher) sendInstancesOrLogError(instancesColl []Instances, err error) {
	if err != nil {
		if trace.IsNotFound(err) {
			return
		}
		slog.ErrorContext(context.Background(), "Failed to fetch instances", "error", err)
		return
	}
	for _, inst := range instancesColl {
		select {
		case w.InstancesC <- inst:
		case <-w.ctx.Done():
		}
	}
}

// fetchAndSubmit fetches the resources and submits them for processing.
func (w *Watcher) fetchAndSubmit() {
	if w.preFetchHookFn != nil {
		w.preFetchHookFn()
	}

	for _, fetcher := range w.fetchersFn() {
		w.sendInstancesOrLogError(fetcher.GetInstances(w.ctx, false))
	}
}

// Run starts the watcher's main watch loop.
func (w *Watcher) Run() {
	pollTimer := w.clock.NewTimer(w.pollInterval)
	defer pollTimer.Stop()

	if w.triggerFetchC == nil {
		w.triggerFetchC = make(<-chan struct{})
	}

	w.fetchAndSubmit()

	for {
		select {
		case insts := <-w.missedRotation:
			for _, fetcher := range w.fetchersFn() {
				w.sendInstancesOrLogError(fetcher.GetMatchingInstances(insts, true))
			}

		case <-pollTimer.Chan():
			w.fetchAndSubmit()
			pollTimer.Reset(w.pollInterval)

		case <-w.triggerFetchC:
			w.fetchAndSubmit()

			// stop and drain timer
			if !pollTimer.Stop() {
				<-pollTimer.Chan()
			}
			pollTimer.Reset(w.pollInterval)

		case <-w.ctx.Done():
			return
		}
	}
}

// Stop stops the watcher.
func (w *Watcher) Stop() {
	w.cancel()
}
