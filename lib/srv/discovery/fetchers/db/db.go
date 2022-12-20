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
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

type makeAWSFetcherFunc func(cloud.AWSClients, string, types.Labels) (common.Fetcher, error)
type makeAzureFetcherFunc func(AzureFetcherConfig) (common.Fetcher, error)

var (
	makeAWSFetcherFuncs = map[string][]makeAWSFetcherFunc{
		services.AWSMatcherRDS:                {makeRDSInstanceFetcher, makeRDSAuroraFetcher},
		services.AWSMatcherRDSProxy:           {makeRDSProxyFetcher},
		services.AWSMatcherRedshift:           {makeRedshiftFetcher},
		services.AWSMatcherRedshiftServerless: {makeRedshiftServerlessFetcher},
		services.AWSMatcherElastiCache:        {makeElastiCacheFetcher},
		services.AWSMatcherMemoryDB:           {makeMemoryDBFetcher},
	}

	makeAzureFetcherFuncs = map[string][]makeAzureFetcherFunc{
		services.AzureMatcherMySQL:     {NewAzureMySQLFetcher},
		services.AzureMatcherPostgres:  {NewAzurePostgresFetcher},
		services.AzureMatcherRedis:     {NewAzureRedisFetcher, NewAzureRedisEnterpriseFetcher},
		services.AzureMatcherSQLServer: {NewAzureSQLServerFetcher, NewAzureManagedSQLServerFetcher},
	}
)

// MakeAWSFetchers creates new AWS database fetchers.
func MakeAWSFetchers(clients cloud.AWSClients, matchers []services.AWSMatcher) (result []common.Fetcher, err error) {
	for _, matcher := range matchers {
		for _, matcherType := range matcher.Types {
			makeFetchers, found := makeAWSFetcherFuncs[matcherType]
			if !found {
				return nil, trace.BadParameter("unknown matcher type %q. Supported AWS matcher types are %v", matcherType, maps.Keys(makeAWSFetcherFuncs))
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

// MakeAzureFetchers creates new Azure database fetchers.
func MakeAzureFetchers(clients cloud.AzureClients, matchers []services.AzureMatcher) (result []common.Fetcher, err error) {
	for _, matcher := range services.SimplifyAzureMatchers(matchers) {
		for _, matcherType := range matcher.Types {
			makeFetchers, found := makeAzureFetcherFuncs[matcherType]
			if !found {
				return nil, trace.BadParameter("unknown matcher type %q. Supported Azure database matcher types are %v", matcherType, maps.Keys(makeAzureFetcherFuncs))
			}

			for _, makeFetcher := range makeFetchers {
				for _, sub := range matcher.Subscriptions {
					for _, group := range matcher.ResourceGroups {
						fetcher, err := makeFetcher(AzureFetcherConfig{
							AzureClients:  clients,
							Type:          matcherType,
							Subscription:  sub,
							ResourceGroup: group,
							Labels:        matcher.ResourceTags,
							Regions:       matcher.Regions,
						})
						if err != nil {
							return nil, trace.Wrap(err)
						}
						result = append(result, fetcher)
					}
				}
			}
		}
	}
	return result, nil
}

// makeRDSInstanceFetcher returns RDS instance fetcher for the provided region and tags.
func makeRDSInstanceFetcher(clients cloud.AWSClients, region string, tags types.Labels) (common.Fetcher, error) {
	rds, err := clients.GetAWSRDSClient(region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fetcher, err := NewRDSDBInstancesFetcher(RDSFetcherConfig{
		Region: region,
		Labels: tags,
		RDS:    rds,
	})
	return fetcher, trace.Wrap(err)
}

// makeRDSAuroraFetcher returns RDS Aurora fetcher for the provided region and tags.
func makeRDSAuroraFetcher(clients cloud.AWSClients, region string, tags types.Labels) (common.Fetcher, error) {
	rds, err := clients.GetAWSRDSClient(region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fetcher, err := NewRDSAuroraClustersFetcher(RDSFetcherConfig{
		Region: region,
		Labels: tags,
		RDS:    rds,
	})
	return fetcher, trace.Wrap(err)
}

// makeRDSProxyFetcher returns RDS proxy fetcher for the provided region and tags.
func makeRDSProxyFetcher(clients cloud.AWSClients, region string, tags types.Labels) (common.Fetcher, error) {
	rds, err := clients.GetAWSRDSClient(region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return NewRDSDBProxyFetcher(RDSFetcherConfig{
		Region: region,
		Labels: tags,
		RDS:    rds,
	})
}

// makeRedshiftFetcher returns Redshift fetcher for the provided region and tags.
func makeRedshiftFetcher(clients cloud.AWSClients, region string, tags types.Labels) (common.Fetcher, error) {
	redshift, err := clients.GetAWSRedshiftClient(region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewRedshiftFetcher(RedshiftFetcherConfig{
		Region:   region,
		Labels:   tags,
		Redshift: redshift,
	})
}

// makeElastiCacheFetcher returns ElastiCache fetcher for the provided region and tags.
func makeElastiCacheFetcher(clients cloud.AWSClients, region string, tags types.Labels) (common.Fetcher, error) {
	elastiCache, err := clients.GetAWSElastiCacheClient(region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewElastiCacheFetcher(ElastiCacheFetcherConfig{
		Region:      region,
		Labels:      tags,
		ElastiCache: elastiCache,
	})
}

// makeMemoryDBFetcher returns MemoryDB fetcher for the provided region and tags.
func makeMemoryDBFetcher(clients cloud.AWSClients, region string, tags types.Labels) (common.Fetcher, error) {
	memorydb, err := clients.GetAWSMemoryDBClient(region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewMemoryDBFetcher(MemoryDBFetcherConfig{
		Region:   region,
		Labels:   tags,
		MemoryDB: memorydb,
	})
}

// makeRedshiftServerlessFetcher returns Redshift Serverless fetcher for the
// provided region and tags.
func makeRedshiftServerlessFetcher(clients cloud.AWSClients, region string, tags types.Labels) (common.Fetcher, error) {
	client, err := clients.GetAWSRedshiftServerlessClient(region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewRedshiftServerlessFetcher(RedshiftServerlessFetcherConfig{
		Region: region,
		Labels: tags,
		Client: client,
	})
}

// filterDatabasesByLabels filters input databases with provided labels.
func filterDatabasesByLabels(databases types.Databases, labels types.Labels, log logrus.FieldLogger) types.Databases {
	var matchedDatabases types.Databases
	for _, database := range databases {
		match, _, err := services.MatchLabels(labels, database.GetAllLabels())
		if err != nil {
			log.Warnf("Failed to match %v against selector: %v.", database, err)
		} else if match {
			matchedDatabases = append(matchedDatabases, database)
		} else {
			log.Debugf("%v doesn't match selector.", database)
		}
	}
	return matchedDatabases
}

// flatten flattens a nested slice [][]T to []T.
func flatten[T any](s [][]T) (result []T) {
	for i := range s {
		result = append(result, s[i]...)
	}
	return
}
