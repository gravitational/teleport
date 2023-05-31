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
	*EC2Instances
	*AzureInstances
}

// Fetcher fetches instances from a particular cloud provider.
type Fetcher interface {
	// GetInstances gets a list of cloud instances.
	GetInstances(ctx context.Context, rotation bool) ([]Instances, error)
	// GetMatchingInstances finds Instances from the list of nodes
	// that the fetcher matches.
	GetMatchingInstances(nodes []types.Server, rotation bool) ([]Instances, error)
}

// Watcher allows callers to discover cloud instances matching specified filters.
type Watcher struct {
	// InstancesC can be used to consume newly discovered instances.
	InstancesC     chan Instances
	missedRotation <-chan []types.Server

	fetchers      []Fetcher
	fetchInterval time.Duration
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

// Run starts the watcher's main watch loop.
func (w *Watcher) Run() {
	if len(w.fetchers) == 0 {
		return
	}
	ticker := time.NewTicker(w.fetchInterval)
	defer ticker.Stop()

	for _, fetcher := range w.fetchers {
		w.sendInstancesOrLogError(fetcher.GetInstances(w.ctx, false))
	}

	for {
		select {
		case insts := <-w.missedRotation:
			for _, fetcher := range w.fetchers {
				w.sendInstancesOrLogError(fetcher.GetMatchingInstances(insts, true))
			}
		case <-ticker.C:
			for _, fetcher := range w.fetchers {
				w.sendInstancesOrLogError(fetcher.GetInstances(w.ctx, false))
			}
		case <-w.ctx.Done():
			return
		}
	}
}

// Stop stops the watcher.
func (w *Watcher) Stop() {
	w.cancel()
}
