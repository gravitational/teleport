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

package common

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/types"
)

const (
	concurrencyLimit = 5
)

// WatcherConfig is the common discovery watcher configuration.
type WatcherConfig struct {
	// Fetchers holds fetchers used for this watcher.
	Fetchers []Fetcher
	// Interval is the interval between fetches.
	Interval time.Duration
	// Log is the watcher logger.
	Log logrus.FieldLogger
	// Clock is used to control time.
	Clock clockwork.Clock
	// DiscoveryGroup is the name of the discovery group that the current
	// discovery service is a part of.
	// It is used to filter out discovered resources that belong to another
	// discovery services. When running in high availability mode and the agents
	// have access to the same cloud resources, this field value must be the same
	// for all discovery services. If different agents are used to discover different
	// sets of cloud resources, this field must be different for each set of agents.
	DiscoveryGroup string
}

// CheckAndSetDefaults validates the config.
func (c *WatcherConfig) CheckAndSetDefaults() error {
	if c.Interval == 0 {
		c.Interval = 5 * time.Minute
	}
	if c.Log == nil {
		c.Log = logrus.New()
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if len(c.Fetchers) == 0 {
		return trace.NotFound("missing fetchers")
	}
	return nil
}

// Watcher monitors cloud resources with provided fetchers.
type Watcher struct {
	// cfg is the watcher config.
	cfg WatcherConfig
	// ctx is the watcher close context.
	ctx context.Context
	// resourcesC is a channel where fetched resourcess are sent.
	resourcesC chan (types.ResourcesWithLabels)
}

// NewWatcher returns a new instance of a common discovery watcher.
func NewWatcher(ctx context.Context, config WatcherConfig) (*Watcher, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Watcher{
		cfg:        config,
		ctx:        ctx,
		resourcesC: make(chan types.ResourcesWithLabels),
	}, nil
}

// Start starts fetching cloud resources and sending them to the channel.
func (w *Watcher) Start() {
	ticker := w.cfg.Clock.NewTicker(w.cfg.Interval)
	defer ticker.Stop()
	w.cfg.Log.Infof("Starting watcher.")
	w.fetchAndSend()
	for {
		select {
		case <-ticker.Chan():
			w.fetchAndSend()
		case <-w.ctx.Done():
			w.cfg.Log.Infof("Watcher done.")
			return
		}
	}
}

// fetchAndSend fetches resources from all fetchers and sends them to the channel.
func (w *Watcher) fetchAndSend() {
	var (
		newFetcherResources = make(types.ResourcesWithLabels, 0, 50)
		fetchersLock        sync.Mutex
		group, groupCtx     = errgroup.WithContext(w.ctx)
	)
	group.SetLimit(concurrencyLimit)
	for _, fetcher := range w.cfg.Fetchers {
		lFetcher := fetcher

		group.Go(func() error {
			resources, err := lFetcher.Get(groupCtx)
			if err != nil {
				// The agent may have permissions to fetch some resources but
				// not others. This is acceptable, so make a debug log instead
				// of a warning.
				if trace.IsAccessDenied(err) || trace.IsNotFound(err) {
					w.cfg.Log.WithError(err).WithField("fetcher", lFetcher).Debugf("Skipped fetcher for %s at %s.", lFetcher.ResourceType(), lFetcher.Cloud())
				} else {
					w.cfg.Log.WithError(err).WithField("fetcher", lFetcher).Warnf("Unable to fetch resources for %s at %s.", lFetcher.ResourceType(), lFetcher.Cloud())
				}
				// never return the error otherwise it will impact other watchers.
				return nil
			}

			for _, r := range resources {
				staticLabels := r.GetStaticLabels()
				if staticLabels == nil {
					staticLabels = make(map[string]string)
				}

				if w.cfg.DiscoveryGroup != "" {
					// Add the discovery group name to the static labels of each resource.
					staticLabels[types.TeleportInternalDiscoveryGroupName] = w.cfg.DiscoveryGroup
				}

				// Set the origin to Cloud indicating that the resource was imported from a cloud provider.
				staticLabels[types.OriginLabel] = types.OriginCloud

				r.SetStaticLabels(staticLabels)
			}

			fetchersLock.Lock()
			newFetcherResources = append(newFetcherResources, resources...)
			fetchersLock.Unlock()
			return nil
		})
	}
	// error is discarded because we must run all fetchers until the end.
	_ = group.Wait()

	select {
	case w.resourcesC <- newFetcherResources:
	case <-w.ctx.Done():
	}
}

// Resources returns a channel that receives fetched cloud resources.
func (w *Watcher) ResourcesC() <-chan types.ResourcesWithLabels {
	return w.resourcesC
}
