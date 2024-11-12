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
		types.AWSMatcherRDS:                {newRDSDBInstancesFetcher, newRDSAuroraClustersFetcher},
		types.AWSMatcherRDSProxy:           {newRDSDBProxyFetcher},
		types.AWSMatcherRedshift:           {newRedshiftFetcher},
		types.AWSMatcherRedshiftServerless: {newRedshiftServerlessFetcher},
		types.AWSMatcherElastiCache:        {newElastiCacheFetcher},
		types.AWSMatcherMemoryDB:           {newMemoryDBFetcher},
		types.AWSMatcherOpenSearch:         {newOpenSearchFetcher},
		types.AWSMatcherDocumentDB:         {newDocumentDBFetcher},
	}

	makeAzureFetcherFuncs = map[string][]makeAzureFetcherFunc{
		types.AzureMatcherMySQL:     {newAzureMySQLFetcher, newAzureMySQLFlexServerFetcher},
		types.AzureMatcherPostgres:  {newAzurePostgresFetcher, newAzurePostgresFlexServerFetcher},
		types.AzureMatcherRedis:     {newAzureRedisFetcher, newAzureRedisEnterpriseFetcher},
		types.AzureMatcherSQLServer: {newAzureSQLServerFetcher, newAzureManagedSQLServerFetcher},
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
						AWSClients:  clients,
						Type:        matcherType,
						AssumeRole:  assumeRole,
						Labels:      matcher.Tags,
						Region:      region,
						Integration: matcher.Integration,
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
