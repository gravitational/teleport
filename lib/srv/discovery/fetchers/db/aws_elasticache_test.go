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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

func TestElastiCacheFetcher(t *testing.T) {
	t.Parallel()

	elasticacheProd, elasticacheDatabasesProd, elasticacheProdTags := makeElastiCacheCluster(t, "ec1", "us-east-1", "prod", mocks.WithElastiCacheReaderEndpoint)
	elasticacheQA, elasticacheDatabasesQA, elasticacheQATags := makeElastiCacheCluster(t, "ec2", "us-east-1", "qa", mocks.WithElastiCacheConfigurationEndpoint, withElastiCacheEngine("valkey"))
	elasticacheUnavailable, _, elasticacheUnavailableTags := makeElastiCacheCluster(t, "ec4", "us-east-1", "prod", func(cluster *ectypes.ReplicationGroup) {
		cluster.Status = aws.String("deleting")
	})
	elasticacheUnsupported, _, elasticacheUnsupportedTags := makeElastiCacheCluster(t, "ec5", "us-east-1", "prod", func(cluster *ectypes.ReplicationGroup) {
		cluster.TransitEncryptionEnabled = aws.Bool(false)
	})
	elasticacheTagsByARN := map[string][]ectypes.Tag{
		aws.ToString(elasticacheProd.ARN):        elasticacheProdTags,
		aws.ToString(elasticacheQA.ARN):          elasticacheQATags,
		aws.ToString(elasticacheUnavailable.ARN): elasticacheUnavailableTags,
		aws.ToString(elasticacheUnsupported.ARN): elasticacheUnsupportedTags,
	}

	tests := []awsFetcherTest{
		{
			name: "fetch all",
			fetcherCfg: AWSFetcherFactoryConfig{
				AWSClients: fakeAWSClients{
					ecClient: &mocks.ElastiCacheClient{
						ReplicationGroups: []ectypes.ReplicationGroup{*elasticacheProd, *elasticacheQA},
						TagsByARN:         elasticacheTagsByARN,
					}},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherElastiCache, "us-east-1", wildcardLabels),
			wantDatabases: append(elasticacheDatabasesProd, elasticacheDatabasesQA...),
		},
		{
			name: "fetch prod",
			fetcherCfg: AWSFetcherFactoryConfig{
				AWSClients: fakeAWSClients{
					ecClient: &mocks.ElastiCacheClient{
						ReplicationGroups: []ectypes.ReplicationGroup{*elasticacheProd, *elasticacheQA},
						TagsByARN:         elasticacheTagsByARN,
					},
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherElastiCache, "us-east-1", envProdLabels),
			wantDatabases: elasticacheDatabasesProd,
		},
		{
			name: "skip unavailable",
			fetcherCfg: AWSFetcherFactoryConfig{
				AWSClients: fakeAWSClients{
					ecClient: &mocks.ElastiCacheClient{
						ReplicationGroups: []ectypes.ReplicationGroup{*elasticacheProd, *elasticacheUnavailable},
						TagsByARN:         elasticacheTagsByARN,
					},
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherElastiCache, "us-east-1", wildcardLabels),
			wantDatabases: elasticacheDatabasesProd,
		},
		{
			name: "skip unsupported",
			fetcherCfg: AWSFetcherFactoryConfig{
				AWSClients: fakeAWSClients{
					ecClient: &mocks.ElastiCacheClient{
						ReplicationGroups: []ectypes.ReplicationGroup{*elasticacheProd, *elasticacheUnsupported},
						TagsByARN:         elasticacheTagsByARN,
					},
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherElastiCache, "us-east-1", wildcardLabels),
			wantDatabases: elasticacheDatabasesProd,
		},
	}
	testAWSFetchers(t, tests...)
}

func makeElastiCacheCluster(t *testing.T, name, region, env string, opts ...func(*ectypes.ReplicationGroup)) (*ectypes.ReplicationGroup, types.Databases, []ectypes.Tag) {
	cluster := mocks.ElastiCacheCluster(name, region, opts...)

	tags := []ectypes.Tag{{
		Key:   aws.String("env"),
		Value: aws.String(env),
	}}
	extraLabels := common.ExtraElastiCacheLabels(cluster, tags, nil, nil)

	if aws.ToBool(cluster.ClusterEnabled) {
		database, err := common.NewDatabaseFromElastiCacheConfigurationEndpoint(cluster, extraLabels)
		require.NoError(t, err)
		common.ApplyAWSDatabaseNameSuffix(database, types.AWSMatcherElastiCache)
		return cluster, types.Databases{database}, tags
	}

	databases, err := common.NewDatabasesFromElastiCacheNodeGroups(cluster, extraLabels)
	require.NoError(t, err)
	for _, database := range databases {
		common.ApplyAWSDatabaseNameSuffix(database, types.AWSMatcherElastiCache)
	}
	return cluster, databases, tags
}

func withElastiCacheEngine(engine string) func(*ectypes.ReplicationGroup) {
	return func(rg *ectypes.ReplicationGroup) {
		rg.Engine = &engine
	}
}
