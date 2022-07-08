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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/cloud/clients"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// WatcherConfig is the cloud watcher configuration.
type WatcherConfig struct {
	// AWSMatchers is a list of matchers for AWS databases.
	AWSMatchers []services.AWSMatcher
	// Clients provides cloud API clients.
	Clients clients.CloudClients
	// Interval is the interval between DB fetches.
	Interval time.Duration
	// EC2Interval is the interval between EC2 fetches
	EC2Interval time.Duration
}

// CheckAndSetDefaults validates the config.
func (c *WatcherConfig) CheckAndSetDefaults() error {
	if c.Clients == nil {
		c.Clients = clients.NewCloudClients()
	}
	if c.Interval == 0 {
		c.Interval = 5 * time.Minute
	}

	if c.EC2Interval == 0 {
		c.EC2Interval = 1 * time.Minute
	}
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
	fetchers []Fetcher
	// databasesC is a channel where fetched databases are sent.
	databasesC chan (types.Databases)
	// ec2C is a channel where fetched ec2 instances are sent.
	ec2C chan ([]EC2Instances)
}

type fetcherKind int

const (
	dbFetcher fetcherKind = iota
	ec2Fetcher
)

// Fetcher fetches cloud databases.
type Fetcher interface {
	// Get returns cloud databases matching the fetcher's selector.
	Get(context.Context) (types.Databases, error)
	// GetEC2Instances returns AWS ec2 instances matching the fetcher's selector
	GetEC2Instances(context.Context) (*EC2Instances, error)

	// Kind returns the type of fetcher to use
	Kind() fetcherKind
}

// NewWatcher returns a new instance of a cloud databases watcher.
func NewWatcher(ctx context.Context, config WatcherConfig) (*Watcher, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	fetchers, err := makeFetchers(config.Clients, config.AWSMatchers)
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
		ec2C:       make(chan []EC2Instances),
	}, nil
}

// Start starts fetching cloud databases and sending them to the channel.
//
// TODO(r0mant): In future, instead of (or in addition to) polling, we can
// use a combination of EventBridge (former CloudWatch Events) and SQS/SNS to
// subscribe to events such as created/removed instances and tag changes, but
// this will require Teleport to have more AWS permissions.
func (w *Watcher) Start() {
	go w.startEC2()
	w.start()
}

func (w *Watcher) start() {
	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()
	w.log.Debugf("Starting cloud databases watcher.")
	w.fetchAndSend()
	for {
		select {
		case <-ticker.C:
			w.fetchAndSend()
		case <-w.ctx.Done():
			w.log.Debugf("Cloud databases and ec2 watcher done.")
			return
		}
	}
}

func (w *Watcher) startEC2() {
	ticker := time.NewTicker(w.cfg.EC2Interval)
	defer ticker.Stop()
	w.log.Debugf("Starting ec2 watcher.")
	w.fetchAndSendEC2()
	for {
		select {
		case <-ticker.C:
			w.fetchAndSendEC2()
		case <-w.ctx.Done():
			w.log.Debugf("Cloud databases and ec2 watcher done.")
			return
		}
	}
}

func (w *Watcher) fetchAndSendEC2() {
	var result []EC2Instances
	for _, fetcher := range w.fetchers {
		if fetcher.Kind() != ec2Fetcher {
			continue
		}
		instances, err := fetcher.GetEC2Instances(w.ctx)
		if err != nil {
			if trace.IsAccessDenied(err) {
				w.log.WithError(err).Debugf("Skipping fetcher %v.", fetcher)
				continue
			}
			w.log.WithError(err).Errorf("%s failed.", fetcher)
			return
		}
		result = append(result, *instances)
	}
	select {
	case w.ec2C <- result:
	case <-w.ctx.Done():
	}
}

// fetchAndSend fetches databases from all fetchers and sends them to the channel.
func (w *Watcher) fetchAndSend() {
	var result types.Databases
	for _, fetcher := range w.fetchers {
		if fetcher.Kind() != dbFetcher {
			continue
		}
		databases, err := fetcher.Get(w.ctx)
		if err != nil {
			// DB agent may have permissions to fetch some databases but not
			// others. This is acceptable, thus continue to other fetchers.
			if trace.IsAccessDenied(err) {
				w.log.WithError(err).Debugf("Skipping fetcher %v.", fetcher)
				continue
			}

			w.log.WithError(err).Errorf("%s failed.", fetcher)
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

func (w *Watcher) EC2C() <-chan []EC2Instances {
	return w.ec2C
}

type fetcherConfig struct {
	region   string
	tags     types.Labels
	document string
	clients  clients.CloudClients
}

// makeFetchers returns cloud fetchers for the provided matchers.
func makeFetchers(clients clients.CloudClients, matchers []services.AWSMatcher) (result []Fetcher, err error) {
	type makeFetcherFunc func(*fetcherConfig) (Fetcher, error)
	makeFetcherFuncs := map[string][]makeFetcherFunc{
		services.AWSMatcherRDS:         {makeRDSInstanceFetcher, makeRDSAuroraFetcher},
		services.AWSMatcherRedshift:    {makeRedshiftFetcher},
		services.AWSMatcherElastiCache: {makeElastiCacheFetcher},
		services.AWSMatcherMemoryDB:    {makeMemoryDBFetcher},
		services.AWSMatcherEC2:         {makeEC2InstanceFetcher},
	}

	for _, matcher := range matchers {
		for _, region := range matcher.Regions {
			for matcherType, makeFetchers := range makeFetcherFuncs {
				if !utils.SliceContainsStr(matcher.Types, matcherType) {
					continue
				}

				for _, makeFetcher := range makeFetchers {
					fetcher, err := makeFetcher(&fetcherConfig{
						clients:  clients,
						region:   region,
						tags:     matcher.Tags,
						document: matcher.SSMDocument,
					})
					if err != nil {
						return nil, trace.Wrap(err)
					}
					result = append(result, fetcher)
				}
			}
		}
	}
	return result, nil
}

// makeRDSInstanceFetcher returns RDS instance fetcher for the provided region and tags.
func makeRDSInstanceFetcher(cfg *fetcherConfig) (Fetcher, error) {
	rds, err := cfg.clients.GetAWSRDSClient(cfg.region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fetcher, err := newRDSDBInstancesFetcher(rdsFetcherConfig{
		Region: cfg.region,
		Labels: cfg.tags,
		RDS:    rds,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return fetcher, nil
}

// makeRDSAuroraFetcher returns RDS Aurora fetcher for the provided region and tags.
func makeRDSAuroraFetcher(cfg *fetcherConfig) (Fetcher, error) {
	rds, err := cfg.clients.GetAWSRDSClient(cfg.region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fetcher, err := newRDSAuroraClustersFetcher(rdsFetcherConfig{
		Region: cfg.region,
		Labels: cfg.tags,
		RDS:    rds,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return fetcher, nil
}

// makeRedshiftFetcher returns Redshift fetcher for the provided region and tags.
func makeRedshiftFetcher(cfg *fetcherConfig) (Fetcher, error) {
	redshift, err := cfg.clients.GetAWSRedshiftClient(cfg.region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newRedshiftFetcher(redshiftFetcherConfig{
		Region:   cfg.region,
		Labels:   cfg.tags,
		Redshift: redshift,
	})
}

// makeElastiCacheFetcher returns ElastiCache fetcher for the provided region and tags.
func makeElastiCacheFetcher(cfg *fetcherConfig) (Fetcher, error) {
	elastiCache, err := cfg.clients.GetAWSElastiCacheClient(cfg.region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newElastiCacheFetcher(elastiCacheFetcherConfig{
		Region:      cfg.region,
		Labels:      cfg.tags,
		ElastiCache: elastiCache,
	})
}

// makeMemoryDBFetcher returns MemoryDB fetcher for the provided region and tags.
func makeMemoryDBFetcher(cfg *fetcherConfig) (Fetcher, error) {
	memorydb, err := cfg.clients.GetAWSMemoryDBClient(cfg.region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newMemoryDBFetcher(memoryDBFetcherConfig{
		Region:   cfg.region,
		Labels:   cfg.tags,
		MemoryDB: memorydb,
	})
}

// makeEC2InstanceFetcher returns MemoryDB fetcher for the provided region and tags.
func makeEC2InstanceFetcher(cfg *fetcherConfig) (Fetcher, error) {
	ec2Client, err := cfg.clients.GetAWSEC2Client(cfg.region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newEc2InstanceFetcher(ec2FetcherConfig{
		Labels:   cfg.tags,
		EC2:      ec2Client,
		Region:   cfg.region,
		Document: cfg.document,
	})
}
