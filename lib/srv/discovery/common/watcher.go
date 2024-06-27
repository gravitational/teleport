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

const (
	// DefaultDiscoveryPollInterval is the default interval that Discovery Services fetches resources.
	DefaultDiscoveryPollInterval = 5 * time.Minute
)

// WatcherConfig is the common discovery watcher configuration.
type WatcherConfig struct {
	// FetchersFn is a function that returns the fetchers used for this watcher.
	FetchersFn func() []Fetcher
	// Interval is the interval between fetches.
	Interval time.Duration
	// TriggerFetchC can be used to force an instant Poll, instead of waiting for the next poll Interval.
	TriggerFetchC chan struct{}
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
	// Origin is used to specify what type of origin watcher's resources are
	Origin string
}

// CheckAndSetDefaults validates the config.
func (c *WatcherConfig) CheckAndSetDefaults() error {
	if c.Interval == 0 {
		c.Interval = DefaultDiscoveryPollInterval
	}
	if c.TriggerFetchC == nil {
		c.TriggerFetchC = make(chan struct{})
	}
	if c.Log == nil {
		c.Log = logrus.New()
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.FetchersFn == nil {
		return trace.NotFound("missing fetchers")
	}
	if c.Origin == "" {
		return trace.BadParameter("origin is not set")
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
	pollTimer := w.cfg.Clock.NewTimer(w.cfg.Interval)
	defer pollTimer.Stop()
	w.cfg.Log.Infof("Starting watcher.")
	w.fetchAndSend()
	for {
		select {
		case <-pollTimer.Chan():
			w.fetchAndSend()
			pollTimer.Reset(w.cfg.Interval)

		case <-w.cfg.TriggerFetchC:
			w.fetchAndSend()

			// stop and drain timer
			if !pollTimer.Stop() {
				<-pollTimer.Chan()
			}
			pollTimer.Reset(w.cfg.Interval)

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
	for _, fetcher := range w.cfg.FetchersFn() {
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

				// Set the origin label to provide information where resource comes from
				staticLabels[types.OriginLabel] = w.cfg.Origin
				if c := lFetcher.Cloud(); c != "" {
					staticLabels[types.CloudLabel] = c
				}

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

// StaticFetchers converts a list of Fetchers into a function that returns them.
// Used to convert a static set of Fetchers into a FetchersFn generator.
func StaticFetchers(fs []Fetcher) func() []Fetcher {
	return func() []Fetcher {
		return fs
	}
}
