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
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

func TestElastiCacheFetcher(t *testing.T) {
	t.Parallel()

	elasticacheProd, elasticacheDatabaseProd, elasticacheProdTags := makeElastiCacheCluster(t, "ec1", "us-east-1", "prod")
	elasticacheQA, elasticacheDatabaseQA, elasticacheQATags := makeElastiCacheCluster(t, "ec2", "us-east-1", "qa", withElastiCacheConfigurationEndpoint())
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
			wantDatabases: types.Databases{elasticacheDatabaseProd, elasticacheDatabaseQA},
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
			wantDatabases: types.Databases{elasticacheDatabaseProd},
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
			wantDatabases: types.Databases{elasticacheDatabaseProd},
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
			wantDatabases: types.Databases{elasticacheDatabaseProd},
		},
	}
	testAWSFetchers(t, tests...)
}

func makeElastiCacheCluster(t *testing.T, name, region, env string, opts ...func(*elasticache.ReplicationGroup)) (*elasticache.ReplicationGroup, types.Database, []*elasticache.Tag) {
	cluster := &elasticache.ReplicationGroup{
		ARN:                      aws.String(fmt.Sprintf("arn:aws:elasticache:%s:123456789012:replicationgroup:%s", region, name)),
		ReplicationGroupId:       aws.String(name),
		Status:                   aws.String("available"),
		TransitEncryptionEnabled: aws.Bool(true),

		// Default has one primary endpoint in the only node group.
		NodeGroups: []*elasticache.NodeGroup{{
			PrimaryEndpoint: &elasticache.Endpoint{
				Address: aws.String("primary.localhost"),
				Port:    aws.Int64(6379),
			},
		}},
	}

	for _, opt := range opts {
		opt(cluster)
	}

	tags := []*elasticache.Tag{{
		Key:   aws.String("env"),
		Value: aws.String(env),
	}}
	extraLabels := services.ExtraElastiCacheLabels(cluster, tags, nil, nil)

	if aws.BoolValue(cluster.ClusterEnabled) {
		database, err := services.NewDatabaseFromElastiCacheConfigurationEndpoint(cluster, extraLabels)
		require.NoError(t, err)
		common.ApplyAWSDatabaseNameSuffix(database, services.AWSMatcherElastiCache)
		return cluster, database, tags
	}

	databases, err := services.NewDatabasesFromElastiCacheNodeGroups(cluster, extraLabels)
	require.NoError(t, err)
	require.Len(t, databases, 1)
	common.ApplyAWSDatabaseNameSuffix(databases[0], services.AWSMatcherElastiCache)
	return cluster, databases[0], tags
}

// withElastiCacheConfigurationEndpoint returns an option function for
// makeElastiCacheCluster to set a configuration endpoint.
func withElastiCacheConfigurationEndpoint() func(*elasticache.ReplicationGroup) {
	return func(cluster *elasticache.ReplicationGroup) {
		cluster.ClusterEnabled = aws.Bool(true)
		cluster.ConfigurationEndpoint = &elasticache.Endpoint{
			Address: aws.String("configuration.localhost"),
			Port:    aws.Int64(6379),
		}
	}
}
