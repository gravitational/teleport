/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
