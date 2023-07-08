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
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

type makeAWSFetcherFunc func(awsFetcherConfig) (common.Fetcher, error)
type makeAzureFetcherFunc func(azureFetcherConfig) (common.Fetcher, error)

var (
	makeAWSFetcherFuncs = map[string][]makeAWSFetcherFunc{
		services.AWSMatcherRDS:                {newRDSDBInstancesFetcher, newRDSAuroraClustersFetcher},
		services.AWSMatcherRDSProxy:           {newRDSDBProxyFetcher},
		services.AWSMatcherRedshift:           {newRedshiftFetcher},
		services.AWSMatcherRedshiftServerless: {newRedshiftServerlessFetcher},
		services.AWSMatcherElastiCache:        {newElastiCacheFetcher},
		services.AWSMatcherMemoryDB:           {newMemoryDBFetcher},
		services.AWSMatcherOpenSearch:         {newOpenSearchFetcher},
	}

	makeAzureFetcherFuncs = map[string][]makeAzureFetcherFunc{
		services.AzureMatcherMySQL:     {newAzureMySQLFetcher, newAzureMySQLFlexServerFetcher},
		services.AzureMatcherPostgres:  {newAzurePostgresFetcher, newAzurePostgresFlexServerFetcher},
		services.AzureMatcherRedis:     {newAzureRedisFetcher, newAzureRedisEnterpriseFetcher},
		services.AzureMatcherSQLServer: {newAzureSQLServerFetcher, newAzureManagedSQLServerFetcher},
	}
)

// IsAWSMatcherType checks if matcher type is a valid AWS matcher.
func IsAWSMatcherType(matcherType string) bool {
	return len(makeAWSFetcherFuncs[matcherType]) > 0
}

// IsAzureMatcherType checks if matcher type is a valid Azure matcher.
func IsAzureMatcherType(matcherType string) bool {
	return len(makeAzureFetcherFuncs[matcherType]) > 0
}

// MakeAWSFetchers creates new AWS database fetchers.
func MakeAWSFetchers(ctx context.Context, clients cloud.AWSClients, matchers []types.AWSMatcher) (result []common.Fetcher, err error) {
	for _, matcher := range matchers {
		assumeRole := types.AssumeRole{}
		if matcher.AssumeRole != nil {
			assumeRole = *matcher.AssumeRole
		}
		for _, matcherType := range matcher.Types {
			makeFetchers, found := makeAWSFetcherFuncs[matcherType]
			if !found {
				return nil, trace.BadParameter("unknown matcher type %q. Supported AWS matcher types are %v", matcherType, maps.Keys(makeAWSFetcherFuncs))
			}

			for _, makeFetcher := range makeFetchers {
				for _, region := range matcher.Regions {
					fetcher, err := makeFetcher(awsFetcherConfig{
						AWSClients: clients,
						Type:       matcherType,
						AssumeRole: assumeRole,
						Labels:     matcher.Tags,
						Region:     region,
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

// MakeAzureFetchers creates new Azure database fetchers.
func MakeAzureFetchers(clients cloud.AzureClients, matchers []types.AzureMatcher) (result []common.Fetcher, err error) {
	for _, matcher := range services.SimplifyAzureMatchers(matchers) {
		for _, matcherType := range matcher.Types {
			makeFetchers, found := makeAzureFetcherFuncs[matcherType]
			if !found {
				return nil, trace.BadParameter("unknown matcher type %q. Supported Azure database matcher types are %v", matcherType, maps.Keys(makeAzureFetcherFuncs))
			}

			for _, makeFetcher := range makeFetchers {
				for _, sub := range matcher.Subscriptions {
					for _, group := range matcher.ResourceGroups {
						fetcher, err := makeFetcher(azureFetcherConfig{
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
