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
	"maps"
	"slices"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// Fetcher fetches instances from a particular cloud provider.
type Fetcher[Instances any] interface {
	// GetInstances gets a list of cloud instances.
	GetInstances(ctx context.Context, rotation bool) ([]Instances, error)
	// GetMatchingInstances finds Instances from the list of nodes
	// that the fetcher matches.
	GetMatchingInstances(ctx context.Context, nodes []types.Server, rotation bool) ([]Instances, error)
	// GetDiscoveryConfigName returns the DiscoveryConfig name that created this fetcher.
	// Empty for Fetchers created from `teleport.yaml/discovery_service.aws.<Matcher>` matchers.
	GetDiscoveryConfigName() string
	// IntegrationName identifies the integration name whose credentials were used to fetch the resources.
	// Might be empty when the fetcher is using ambient credentials.
	IntegrationName() string
}

// Option is a functional option for the Watcher.
type Option[Instances any] func(*Watcher[Instances])

// WithPollInterval sets the interval at which the watcher will fetch
// instances.
func WithPollInterval[Instances any](interval time.Duration) Option[Instances] {
	return func(w *Watcher[Instances]) {
		w.pollInterval = interval
	}
}

// WithTriggerFetchC sets a poll trigger to manual start a resource polling.
func WithTriggerFetchC[Instances any](triggerFetchC <-chan struct{}) Option[Instances] {
	return func(w *Watcher[Instances]) {
		w.triggerFetchC = triggerFetchC
	}
}

// WithTriggerFetchHookFn sets a callback function to call each time the watcher receives a manual trigger.
// The hook is called prior to processing the update.
func WithTriggerFetchHookFn[Instances any](callback func()) Option[Instances] {
	return func(w *Watcher[Instances]) {
		w.triggerFetchHookFn = callback
	}
}

// WithPreFetchHookFn sets a function that gets called before each new iteration.
func WithPreFetchHookFn[Instances any](f func(fetchers []Fetcher[Instances])) Option[Instances] {
	return func(w *Watcher[Instances]) {
		w.preFetchHookFn = f
	}
}

// WithPerInstanceHookFn sets an optional callback for each fetched set of group of instances.
// It will be called once per each fetcher.
// This callback replaces normal channel writes done to InstancesC.
func WithPerInstanceHookFn[Instances any](callback func(groups []Instances)) Option[Instances] {
	return func(w *Watcher[Instances]) {
		w.perInstanceHookFn = callback
	}
}

// WithPostFetchHookFn sets an optional callback to be called after the fetch round is finished.
func WithPostFetchHookFn[Instances any](f func()) Option[Instances] {
	return func(w *Watcher[Instances]) {
		w.postFetchHookFn = f
	}
}

// WithClock sets a clock that is used to periodically fetch new resources.
func WithClock[Instances any](clock clockwork.Clock) Option[Instances] {
	return func(w *Watcher[Instances]) {
		w.clock = clock
	}
}

// WithMissedRotation sets the missed rotation channel.
// Specialized for EC2Instances since this functionality is specific to EC2 servers.
func WithMissedRotation(missedRotation <-chan []types.Server) Option[*EC2Instances] {
	return func(w *Watcher[*EC2Instances]) {
		w.missedRotation = missedRotation
	}
}

// Watcher allows callers to discover cloud instances matching specified filters.
type Watcher[Instances any] struct {
	// InstancesC can be used to consume newly discovered instances.
	InstancesC     chan Instances
	missedRotation <-chan []types.Server

	fetcherMap utils.SyncMap[string, []Fetcher[Instances]]

	pollInterval       time.Duration
	clock              clockwork.Clock
	triggerFetchC      <-chan struct{}
	ctx                context.Context
	cancel             context.CancelFunc
	preFetchHookFn     func(fetchers []Fetcher[Instances])
	postFetchHookFn    func()
	triggerFetchHookFn func()
	perInstanceHookFn  func(instances []Instances)
}

// NewWatcher initializes a new instance of Watcher.
func NewWatcher[Instances any](ctx context.Context, opts ...Option[Instances]) *Watcher[Instances] {
	cancelCtx, cancelFn := context.WithCancel(ctx)
	watcher := Watcher[Instances]{
		ctx:          cancelCtx,
		cancel:       cancelFn,
		clock:        clockwork.NewRealClock(),
		pollInterval: time.Minute,
		InstancesC:   make(chan Instances),
	}
	watcher.perInstanceHookFn = func(instances []Instances) {
		for _, inst := range instances {
			if cancelCtx.Err() != nil {
				return
			}

			select {
			case watcher.InstancesC <- inst:
			case <-cancelCtx.Done():
			}
		}
	}

	for _, opt := range opts {
		opt(&watcher)
	}
	return &watcher
}

// SetFetchers sets the fetcher set for a given discovery config.
func (w *Watcher[Instances]) SetFetchers(dcName string, fetchers []Fetcher[Instances]) {
	w.fetcherMap.Store(dcName, fetchers)
}

// DeleteFetchers removes the fetchers for a given discovery config.
func (w *Watcher[Instances]) DeleteFetchers(dcName string) {
	w.fetcherMap.Delete(dcName)
}

// ReplaceFetchers replaces whole fetcher set atomically.
func (w *Watcher[Instances]) ReplaceFetchers(replaceMap map[string][]Fetcher[Instances]) {
	w.fetcherMap.Set(replaceMap)
}

func (w *Watcher[Instances]) sendInstancesOrLogError(instancesColl []Instances, err error) {
	if err != nil {
		if trace.IsNotFound(err) {
			return
		}
		slog.ErrorContext(context.Background(), "Failed to fetch instances", "error", err)
		return
	}
	w.perInstanceHookFn(instancesColl)
}

// fetchAndSubmit fetches the resources and submits them for processing.
func (w *Watcher[Instances]) fetchAndSubmit() {
	cloned := w.fetcherMap.Clone()
	fetchers := slices.Concat(slices.Collect(maps.Values(cloned))...)

	if w.preFetchHookFn != nil {
		w.preFetchHookFn(fetchers)
	}

	for _, fetcher := range fetchers {
		w.sendInstancesOrLogError(fetcher.GetInstances(w.ctx, false))
	}

	if w.postFetchHookFn != nil {
		w.postFetchHookFn()
	}
}

// Run starts the watcher's main watch loop.
func (w *Watcher[Instances]) Run() {
	pollTimer := w.clock.NewTimer(w.pollInterval)
	defer pollTimer.Stop()

	w.fetchAndSubmit()

	for {
		select {
		case insts := <-w.missedRotation:
			cloned := w.fetcherMap.Clone()
			fetchers := slices.Concat(slices.Collect(maps.Values(cloned))...)

			for _, fetcher := range fetchers {
				w.sendInstancesOrLogError(fetcher.GetMatchingInstances(w.ctx, insts, true))
			}

		case <-pollTimer.Chan():
			w.fetchAndSubmit()
			pollTimer.Reset(w.pollInterval)

		case <-w.triggerFetchC:
			if w.triggerFetchHookFn != nil {
				w.triggerFetchHookFn()
			}

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
func (w *Watcher[Instances]) Stop() {
	w.cancel()
}
