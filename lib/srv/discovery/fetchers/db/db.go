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
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	"github.com/gravitational/trace"
	"golang.org/x/exp/maps"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
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

// AWSFetcherFactoryConfig is the configuration for an [AWSFetcherFactory].
type AWSFetcherFactoryConfig struct {
	// AWSConfigProvider provides [aws.Config] for AWS SDK service clients.
	AWSConfigProvider awsconfig.Provider
	// CloudClients is an interface for retrieving AWS SDK v1 cloud clients.
	CloudClients cloud.AWSClients
	// IntegrationCredentialProviderFn is an optional function that provides
	// credentials via AWS OIDC integration.
	IntegrationCredentialProviderFn awsconfig.IntegrationCredentialProviderFunc
	// RedshiftClientProviderFn is an optional function that provides
	RedshiftClientProviderFn RedshiftClientProviderFunc
}

func (c *AWSFetcherFactoryConfig) checkAndSetDefaults() error {
	if c.CloudClients == nil {
		return trace.BadParameter("missing CloudClients")
	}
	if c.AWSConfigProvider == nil {
		return trace.BadParameter("missing AWSConfigProvider")
	}
	if c.RedshiftClientProviderFn == nil {
		c.RedshiftClientProviderFn = func(cfg aws.Config, optFns ...func(*redshift.Options)) RedshiftClient {
			return redshift.NewFromConfig(cfg, optFns...)
		}
	}
	return nil
}

// AWSFetcherFactory makes AWS database fetchers.
type AWSFetcherFactory struct {
	cfg AWSFetcherFactoryConfig
}

// NewAWSFetcherFactory checks the given config and returns a new fetcher
// provider.
func NewAWSFetcherFactory(cfg AWSFetcherFactoryConfig) (*AWSFetcherFactory, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &AWSFetcherFactory{
		cfg: cfg,
	}, nil
}

// MakeFetchers returns AWS database fetchers for each matcher given.
func (f *AWSFetcherFactory) MakeFetchers(ctx context.Context, matchers []types.AWSMatcher, discoveryConfigName string) ([]common.Fetcher, error) {
	var result []common.Fetcher
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
						AWSClients:                      f.cfg.CloudClients,
						Type:                            matcherType,
						AssumeRole:                      assumeRole,
						Labels:                          matcher.Tags,
						Region:                          region,
						Integration:                     matcher.Integration,
						DiscoveryConfigName:             discoveryConfigName,
						AWSConfigProvider:               f.cfg.AWSConfigProvider,
						IntegrationCredentialProviderFn: f.cfg.IntegrationCredentialProviderFn,
						redshiftClientProviderFn:        f.cfg.RedshiftClientProviderFn,
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
func MakeAzureFetchers(clients cloud.AzureClients, matchers []types.AzureMatcher, discoveryConfigName string) (result []common.Fetcher, err error) {
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
							AzureClients:        clients,
							Type:                matcherType,
							Subscription:        sub,
							ResourceGroup:       group,
							Labels:              matcher.ResourceTags,
							Regions:             matcher.Regions,
							DiscoveryConfigName: discoveryConfigName,
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
func filterDatabasesByLabels(ctx context.Context, databases types.Databases, labels types.Labels, logger *slog.Logger) types.Databases {
	var matchedDatabases types.Databases
	for _, database := range databases {
		match, _, err := services.MatchLabels(labels, database.GetAllLabels())
		if err != nil {
			logger.WarnContext(ctx, "Failed to match database gainst selector", "database", database, "error", err)
		} else if match {
			matchedDatabases = append(matchedDatabases, database)
		} else {
			logger.DebugContext(ctx, "database doesn't match selector", "database", database)
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
