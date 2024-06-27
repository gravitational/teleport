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
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

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
}

// WithTriggerFetchC sets a poll trigger to manual start a resource polling.
func WithTriggerFetchC(triggerFetchC <-chan struct{}) Option {
	return func(w *Watcher) {
		w.triggerFetchC = triggerFetchC
	}
}

// Watcher allows callers to discover cloud instances matching specified filters.
type Watcher struct {
	// InstancesC can be used to consume newly discovered instances.
	InstancesC     chan Instances
	missedRotation <-chan []types.Server

	fetchersFn    func() []Fetcher
	pollInterval  time.Duration
	triggerFetchC <-chan struct{}
	ctx           context.Context
	cancel        context.CancelFunc
}

func (w *Watcher) sendInstancesOrLogError(instancesColl []Instances, err error) {
	if err != nil {
		if trace.IsNotFound(err) {
			return
		}
		log.WithError(err).Error("Failed to fetch instances")
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
	for _, fetcher := range w.fetchersFn() {
		w.sendInstancesOrLogError(fetcher.GetInstances(w.ctx, false))
	}
}

// Run starts the watcher's main watch loop.
func (w *Watcher) Run() {
	pollTimer := time.NewTimer(w.pollInterval)
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

		case <-pollTimer.C:
			w.fetchAndSubmit()
			pollTimer.Reset(w.pollInterval)

		case <-w.triggerFetchC:
			w.fetchAndSubmit()

			// stop and drain timer
			if !pollTimer.Stop() {
				<-pollTimer.C
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
