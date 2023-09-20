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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/services"
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
			inputMatchers: makeAWSMatchersForType(services.AWSMatcherElastiCache, "us-east-1", wildcardLabels),
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
			inputMatchers: makeAWSMatchersForType(services.AWSMatcherElastiCache, "us-east-1", envProdLabels),
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
			inputMatchers: makeAWSMatchersForType(services.AWSMatcherElastiCache, "us-east-1", wildcardLabels),
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
			inputMatchers: makeAWSMatchersForType(services.AWSMatcherElastiCache, "us-east-1", wildcardLabels),
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
	extraLabels := services.ExtraElastiCacheLabels(cluster, tags, nil, nil)

	if aws.BoolValue(cluster.ClusterEnabled) {
		database, err := services.NewDatabaseFromElastiCacheConfigurationEndpoint(cluster, extraLabels)
		require.NoError(t, err)
		return cluster, types.Databases{database}, tags
	}

	databases, err := services.NewDatabasesFromElastiCacheNodeGroups(cluster, extraLabels)
	require.NoError(t, err)
	return cluster, databases, tags
}
