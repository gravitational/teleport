/*
Copyright 2021 Gravitational, Inc.

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

package watchers

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/services"
	discovery "github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/srv/discovery/fetchers/db"
)

// WatcherConfig is the cloud watcher configuration.
type WatcherConfig struct {
	// AWSMatchers is a list of matchers for AWS databases.
	AWSMatchers []services.AWSMatcher
	// AzureMatchers is a list of matchers for Azure databases.
	AzureMatchers []services.AzureMatcher
	// Clients provides cloud API clients.
	Clients cloud.Clients
	// Interval is the interval between fetches.
	Interval time.Duration
}

// CheckAndSetDefaults validates the config.
func (c *WatcherConfig) CheckAndSetDefaults() error {
	if c.Clients == nil {
		c.Clients = cloud.NewClients()
	}
	if c.Interval == 0 {
		c.Interval = 5 * time.Minute
	}
	c.AzureMatchers = db.SimplifyMatchers(c.AzureMatchers)
	return nil
}

// Watcher monitors cloud databases according to the provided selectors.
type Watcher struct {
	// cfg is the watcher config.
	cfg WatcherConfig
	// log is the watcher logger.
	log logrus.FieldLogger
	// ctx is the watcher close context.
	ctx context.Context
	// fetchers fetch databases according to their selectors.
	fetchers []discovery.Fetcher
	// databasesC is a channel where fetched databases are sent.
	databasesC chan (types.Databases)
}

// NewWatcher returns a new instance of a cloud databases watcher.
func NewWatcher(ctx context.Context, config WatcherConfig) (*Watcher, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	fetchers, err := makeFetchers(ctx, &config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(fetchers) == 0 {
		return nil, trace.NotFound("no cloud selectors")
	}
	return &Watcher{
		cfg:        config,
		log:        logrus.WithField(trace.Component, "watcher:cloud"),
		ctx:        ctx,
		fetchers:   fetchers,
		databasesC: make(chan types.Databases),
	}, nil
}

// Start starts fetching cloud databases and sending them to the channel.
//
// TODO(r0mant): In future, instead of (or in addition to) polling, we can
// use a combination of EventBridge (former CloudWatch Events) and SQS/SNS to
// subscribe to events such as created/removed instances and tag changes, but
// this will require Teleport to have more AWS permissions.
func (w *Watcher) Start() {
	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()
	w.log.Debugf("Starting cloud databases watcher.")
	w.fetchAndSend()
	for {
		select {
		case <-ticker.C:
			w.fetchAndSend()
		case <-w.ctx.Done():
			w.log.Debugf("Cloud databases watcher done.")
			return
		}
	}
}

// fetchAndSend fetches databases from all fetchers and sends them to the channel.
func (w *Watcher) fetchAndSend() {
	var result types.Databases
	for _, fetcher := range w.fetchers {
		resources, err := fetcher.Get(w.ctx)
		if err != nil {
			// DB agent may have permissions to fetch some databases but not
			// others. This is acceptable, thus continue to other fetchers.
			// DB agent may also query for resources that do not exist. This is ok.
			// If the resource is created in the future, we will fetch it then.
			if trace.IsAccessDenied(err) || trace.IsNotFound(err) {
				w.log.WithError(err).Debugf("Skipping fetcher %v.", fetcher)
				continue
			}

			w.log.WithError(err).Errorf("%s failed.", fetcher)
			return
		}
		databases, err := resources.AsDatabases()
		if err != nil {
			w.log.WithError(err).Errorf("Failed to convert types.ResourcesWithLabels to types.Databases for fetcher %v.", fetcher)
			return
		}
		result = append(result, databases...)
	}
	select {
	case w.databasesC <- result:
	case <-w.ctx.Done():
	}
}

// DatabasesC returns a channel that receives fetched cloud databases.
func (w *Watcher) DatabasesC() <-chan types.Databases {
	return w.databasesC
}

// makeFetchers returns cloud fetchers for the provided matchers.
func makeFetchers(ctx context.Context, config *WatcherConfig) (result []discovery.Fetcher, err error) {
	fetchers, err := db.MakeAWSFetchers(config.Clients, config.AWSMatchers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	result = append(result, fetchers...)

	fetchers, err = db.MakeAzureFetchers(ctx, config.Clients, config.AzureMatchers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	result = append(result, fetchers...)

	return result, nil
}
