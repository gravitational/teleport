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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

func TestElastiCacheFetcher(t *testing.T) {
	t.Parallel()

	elasticacheProd, elasticacheDatabasesProd, elasticacheProdTags := makeElastiCacheCluster(t, "ec1", "us-east-1", "prod", mocks.WithElastiCacheReaderEndpoint)
	elasticacheQA, elasticacheDatabasesQA, elasticacheQATags := makeElastiCacheCluster(t, "ec2", "us-east-1", "qa", mocks.WithElastiCacheConfigurationEndpoint)
	elasticacheUnavailable, _, elasticacheUnavailableTags := makeElastiCacheCluster(t, "ec4", "us-east-1", "prod", func(cluster *elasticache.ReplicationGroup) {
		cluster.Status = aws.String("deleting")
	})
	elasticacheUnsupported, _, elasticacheUnsupportedTags := makeElastiCacheCluster(t, "ec5", "us-east-1", "prod", func(cluster *elasticache.ReplicationGroup) {
		cluster.TransitEncryptionEnabled = aws.Bool(false)
	})
	elasticacheTagsByARN := map[string][]*elasticache.Tag{
		aws.StringValue(elasticacheProd.ARN):        elasticacheProdTags,
		aws.StringValue(elasticacheQA.ARN):          elasticacheQATags,
		aws.StringValue(elasticacheUnavailable.ARN): elasticacheUnavailableTags,
		aws.StringValue(elasticacheUnsupported.ARN): elasticacheUnsupportedTags,
	}

	tests := []awsFetcherTest{
		{
			name: "fetch all",
			inputClients: &cloud.TestCloudClients{
				ElastiCache: &mocks.ElastiCacheMock{
					ReplicationGroups: []*elasticache.ReplicationGroup{elasticacheProd, elasticacheQA},
					TagsByARN:         elasticacheTagsByARN,
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherElastiCache, "us-east-1", wildcardLabels),
			wantDatabases: append(elasticacheDatabasesProd, elasticacheDatabasesQA...),
		},
		{
			name: "fetch prod",
			inputClients: &cloud.TestCloudClients{
				ElastiCache: &mocks.ElastiCacheMock{
					ReplicationGroups: []*elasticache.ReplicationGroup{elasticacheProd, elasticacheQA},
					TagsByARN:         elasticacheTagsByARN,
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherElastiCache, "us-east-1", envProdLabels),
			wantDatabases: elasticacheDatabasesProd,
		},
		{
			name: "skip unavailable",
			inputClients: &cloud.TestCloudClients{
				ElastiCache: &mocks.ElastiCacheMock{
					ReplicationGroups: []*elasticache.ReplicationGroup{elasticacheProd, elasticacheUnavailable},
					TagsByARN:         elasticacheTagsByARN,
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherElastiCache, "us-east-1", wildcardLabels),
			wantDatabases: elasticacheDatabasesProd,
		},
		{
			name: "skip unsupported",
			inputClients: &cloud.TestCloudClients{
				ElastiCache: &mocks.ElastiCacheMock{
					ReplicationGroups: []*elasticache.ReplicationGroup{elasticacheProd, elasticacheUnsupported},
					TagsByARN:         elasticacheTagsByARN,
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherElastiCache, "us-east-1", wildcardLabels),
			wantDatabases: elasticacheDatabasesProd,
		},
	}
	testAWSFetchers(t, tests...)
}

func makeElastiCacheCluster(t *testing.T, name, region, env string, opts ...func(*elasticache.ReplicationGroup)) (*elasticache.ReplicationGroup, types.Databases, []*elasticache.Tag) {
	cluster := mocks.ElastiCacheCluster(name, region, opts...)

	tags := []*elasticache.Tag{{
		Key:   aws.String("env"),
		Value: aws.String(env),
	}}
	extraLabels := common.ExtraElastiCacheLabels(cluster, tags, nil, nil)

	if aws.BoolValue(cluster.ClusterEnabled) {
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
