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

type makeAWSFetcherFunc func(context.Context, cloud.AWSClients, string, types.Labels, types.AssumeRole) (common.Fetcher, error)
type makeAzureFetcherFunc func(azureFetcherConfig) (common.Fetcher, error)

var (
	makeAWSFetcherFuncs = map[string][]makeAWSFetcherFunc{
		services.AWSMatcherRDS:                {makeRDSInstanceFetcher, makeRDSAuroraFetcher},
		services.AWSMatcherRDSProxy:           {makeRDSProxyFetcher},
		services.AWSMatcherRedshift:           {makeRedshiftFetcher},
		services.AWSMatcherRedshiftServerless: {makeRedshiftServerlessFetcher},
		services.AWSMatcherElastiCache:        {makeElastiCacheFetcher},
		services.AWSMatcherMemoryDB:           {makeMemoryDBFetcher},
		services.AWSMatcherOpenSearch:         {makeOpenSearchFetcher},
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
					fetcher, err := makeFetcher(ctx, clients, region, matcher.Tags, assumeRole)
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

// makeRDSInstanceFetcher returns RDS instance fetcher for the provided region and tags.
func makeRDSInstanceFetcher(ctx context.Context, clients cloud.AWSClients, region string, tags types.Labels, assumeRole types.AssumeRole) (common.Fetcher, error) {
	rds, err := clients.GetAWSRDSClient(ctx, region, cloud.WithAssumeRole(assumeRole.RoleARN, assumeRole.ExternalID))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fetcher, err := newRDSDBInstancesFetcher(rdsFetcherConfig{
		Region:     region,
		Labels:     tags,
		RDS:        rds,
		AssumeRole: assumeRole,
	})
	return fetcher, trace.Wrap(err)
}

// makeRDSAuroraFetcher returns RDS Aurora fetcher for the provided region and tags.
func makeRDSAuroraFetcher(ctx context.Context, clients cloud.AWSClients, region string, tags types.Labels, assumeRole types.AssumeRole) (common.Fetcher, error) {
	rds, err := clients.GetAWSRDSClient(ctx, region, cloud.WithAssumeRole(assumeRole.RoleARN, assumeRole.ExternalID))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fetcher, err := newRDSAuroraClustersFetcher(rdsFetcherConfig{
		Region:     region,
		Labels:     tags,
		RDS:        rds,
		AssumeRole: assumeRole,
	})
	return fetcher, trace.Wrap(err)
}

// makeRDSProxyFetcher returns RDS proxy fetcher for the provided region and tags.
func makeRDSProxyFetcher(ctx context.Context, clients cloud.AWSClients, region string, tags types.Labels, assumeRole types.AssumeRole) (common.Fetcher, error) {
	rds, err := clients.GetAWSRDSClient(ctx, region, cloud.WithAssumeRole(assumeRole.RoleARN, assumeRole.ExternalID))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newRDSDBProxyFetcher(rdsFetcherConfig{
		Region:     region,
		Labels:     tags,
		RDS:        rds,
		AssumeRole: assumeRole,
	})
}

// makeRedshiftFetcher returns Redshift fetcher for the provided region and tags.
func makeRedshiftFetcher(ctx context.Context, clients cloud.AWSClients, region string, tags types.Labels, assumeRole types.AssumeRole) (common.Fetcher, error) {
	redshift, err := clients.GetAWSRedshiftClient(ctx, region, cloud.WithAssumeRole(assumeRole.RoleARN, assumeRole.ExternalID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newRedshiftFetcher(redshiftFetcherConfig{
		Region:     region,
		Labels:     tags,
		Redshift:   redshift,
		AssumeRole: assumeRole,
	})
}

// makeElastiCacheFetcher returns ElastiCache fetcher for the provided region and tags.
func makeElastiCacheFetcher(ctx context.Context, clients cloud.AWSClients, region string, tags types.Labels, assumeRole types.AssumeRole) (common.Fetcher, error) {
	elastiCache, err := clients.GetAWSElastiCacheClient(ctx, region, cloud.WithAssumeRole(assumeRole.RoleARN, assumeRole.ExternalID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newElastiCacheFetcher(elastiCacheFetcherConfig{
		Region:      region,
		Labels:      tags,
		ElastiCache: elastiCache,
		AssumeRole:  assumeRole,
	})
}

// makeMemoryDBFetcher returns MemoryDB fetcher for the provided region and tags.
func makeMemoryDBFetcher(ctx context.Context, clients cloud.AWSClients, region string, tags types.Labels, assumeRole types.AssumeRole) (common.Fetcher, error) {
	memorydb, err := clients.GetAWSMemoryDBClient(ctx, region, cloud.WithAssumeRole(assumeRole.RoleARN, assumeRole.ExternalID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newMemoryDBFetcher(memoryDBFetcherConfig{
		Region:     region,
		Labels:     tags,
		MemoryDB:   memorydb,
		AssumeRole: assumeRole,
	})
}

// makeOpenSearchFetcher returns OpenSearch fetcher for the provided region and tags.
func makeOpenSearchFetcher(ctx context.Context, clients cloud.AWSClients, region string, tags types.Labels, assumeRole types.AssumeRole) (common.Fetcher, error) {
	opensearch, err := clients.GetAWSOpenSearchClient(ctx, region, cloud.WithAssumeRole(assumeRole.RoleARN, assumeRole.ExternalID))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newOpenSearchFetcher(openSearchFetcherConfig{
		Region:     region,
		Labels:     tags,
		openSearch: opensearch,
		AssumeRole: assumeRole,
	})
}

// makeRedshiftServerlessFetcher returns Redshift Serverless fetcher for the
// provided region and tags.
func makeRedshiftServerlessFetcher(ctx context.Context, clients cloud.AWSClients, region string, tags types.Labels, assumeRole types.AssumeRole) (common.Fetcher, error) {
	client, err := clients.GetAWSRedshiftServerlessClient(ctx, region, cloud.WithAssumeRole(assumeRole.RoleARN, assumeRole.ExternalID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newRedshiftServerlessFetcher(redshiftServerlessFetcherConfig{
		Region:     region,
		Labels:     tags,
		Client:     client,
		AssumeRole: assumeRole,
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

// applyAssumeRoleToDatabases applies assume role settings from fetcher to databases.
func applyAssumeRoleToDatabases(databases types.Databases, assumeRole types.AssumeRole) {
	for _, db := range databases {
		db.SetAWSAssumeRole(assumeRole.RoleARN)
		db.SetAWSExternalID(assumeRole.ExternalID)
	}
}

// flatten flattens a nested slice [][]T to []T.
func flatten[T any](s [][]T) (result []T) {
	for i := range s {
		result = append(result, s[i]...)
	}
	return
}
