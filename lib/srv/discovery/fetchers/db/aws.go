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

package db

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// MakeAWSFetchers creates fetchers for AWS-hosted databases.
func MakeAWSFetchers(clients cloud.Clients, matchers []services.AWSMatcher) (result []common.Fetcher, err error) {
	type makeFetcherFunc func(cloud.Clients, string, types.Labels) (common.Fetcher, error)
	makeFetcherFuncs := map[string][]makeFetcherFunc{
		services.AWSMatcherRDS:         {makeRDSInstanceFetcher, makeRDSAuroraFetcher},
		services.AWSMatcherRDSProxy:    {makeRDSProxyFetcher},
		services.AWSMatcherRedshift:    {makeRedshiftFetcher},
		services.AWSMatcherElastiCache: {makeElastiCacheFetcher},
		services.AWSMatcherMemoryDB:    {makeMemoryDBFetcher},
	}

	for _, matcher := range matchers {
		for _, matcherType := range matcher.Types {
			makeFetchers, found := makeFetcherFuncs[matcherType]
			if !found {
				return nil, trace.BadParameter("unknown matcher type %q", matcherType)
			}

			for _, makeFetcher := range makeFetchers {
				for _, region := range matcher.Regions {
					fetcher, err := makeFetcher(clients, region, matcher.Tags)
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
func makeRDSInstanceFetcher(clients cloud.Clients, region string, tags types.Labels) (common.Fetcher, error) {
	rds, err := clients.GetAWSRDSClient(region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fetcher, err := newRDSDBInstancesFetcher(rdsFetcherConfig{
		Region: region,
		Labels: tags,
		RDS:    rds,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return fetcher, nil
}

// makeRDSAuroraFetcher returns RDS Aurora fetcher for the provided region and tags.
func makeRDSAuroraFetcher(clients cloud.Clients, region string, tags types.Labels) (common.Fetcher, error) {
	rds, err := clients.GetAWSRDSClient(region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fetcher, err := newRDSAuroraClustersFetcher(rdsFetcherConfig{
		Region: region,
		Labels: tags,
		RDS:    rds,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return fetcher, nil
}

// makeRDSProxyFetcher returns RDS proxy fetcher for the provided region and tags.
func makeRDSProxyFetcher(clients cloud.Clients, region string, tags types.Labels) (common.Fetcher, error) {
	rds, err := clients.GetAWSRDSClient(region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newRDSDBProxyFetcher(rdsFetcherConfig{
		Region: region,
		Labels: tags,
		RDS:    rds,
	})
}

// makeRedshiftFetcher returns Redshift fetcher for the provided region and tags.
func makeRedshiftFetcher(clients cloud.Clients, region string, tags types.Labels) (common.Fetcher, error) {
	redshift, err := clients.GetAWSRedshiftClient(region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newRedshiftFetcher(redshiftFetcherConfig{
		Region:   region,
		Labels:   tags,
		Redshift: redshift,
	})
}

// makeElastiCacheFetcher returns ElastiCache fetcher for the provided region and tags.
func makeElastiCacheFetcher(clients cloud.Clients, region string, tags types.Labels) (common.Fetcher, error) {
	elastiCache, err := clients.GetAWSElastiCacheClient(region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newElastiCacheFetcher(elastiCacheFetcherConfig{
		Region:      region,
		Labels:      tags,
		ElastiCache: elastiCache,
	})
}

// makeMemoryDBFetcher returns MemoryDB fetcher for the provided region and tags.
func makeMemoryDBFetcher(clients cloud.Clients, region string, tags types.Labels) (common.Fetcher, error) {
	memorydb, err := clients.GetAWSMemoryDBClient(region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newMemoryDBFetcher(memoryDBFetcherConfig{
		Region:   region,
		Labels:   tags,
		MemoryDB: memorydb,
	})
}

// awsFetcher is a common base struct for all AWS database fetchers.
type awsFetcher struct {
}

// ResourceType identifies the resource type the fetcher is returning.
func (f awsFetcher) ResourceType() string {
	return types.KindDatabase
}

// Cloud returns the cloud the fetcher is operating.
func (a awsFetcher) Cloud() string {
	return types.CloudAWS
}

// maxAWSPages is the maximum number of pages to iterate over when fetching AWS databases.
const maxAWSPages = 10
