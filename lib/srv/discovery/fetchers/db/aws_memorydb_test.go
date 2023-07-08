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
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

func TestMemoryDBFetcher(t *testing.T) {
	t.Parallel()

	memorydbProd, memorydbDatabaseProd, memorydbProdTags := makeMemoryDBCluster(t, "memory1", "us-east-1", "prod")
	memorydbTest, memorydbDatabaseTest, memorydbTestTags := makeMemoryDBCluster(t, "memory2", "us-east-1", "test")
	memorydbUnavailable, _, memorydbUnavailableTags := makeMemoryDBCluster(t, "memory3", "us-east-1", "prod", func(cluster *memorydb.Cluster) {
		cluster.Status = aws.String("deleting")
	})
	memorydbUnsupported, _, memorydbUnsupportedTags := makeMemoryDBCluster(t, "memory4", "us-east-1", "prod", func(cluster *memorydb.Cluster) {
		cluster.TLSEnabled = aws.Bool(false)
	})
	memorydbTagsByARN := map[string][]*memorydb.Tag{
		aws.StringValue(memorydbProd.ARN):        memorydbProdTags,
		aws.StringValue(memorydbTest.ARN):        memorydbTestTags,
		aws.StringValue(memorydbUnavailable.ARN): memorydbUnavailableTags,
		aws.StringValue(memorydbUnsupported.ARN): memorydbUnsupportedTags,
	}

	tests := []awsFetcherTest{
		{
			name: "fetch all",
			inputClients: &cloud.TestCloudClients{
				MemoryDB: &mocks.MemoryDBMock{
					Clusters:  []*memorydb.Cluster{memorydbProd, memorydbTest},
					TagsByARN: memorydbTagsByARN,
				},
			},
			inputMatchers: makeAWSMatchersForType(services.AWSMatcherMemoryDB, "us-east-1", wildcardLabels),
			wantDatabases: types.Databases{memorydbDatabaseProd, memorydbDatabaseTest},
		},
		{
			name: "fetch prod",
			inputClients: &cloud.TestCloudClients{
				MemoryDB: &mocks.MemoryDBMock{
					Clusters:  []*memorydb.Cluster{memorydbProd, memorydbTest},
					TagsByARN: memorydbTagsByARN,
				},
			},
			inputMatchers: makeAWSMatchersForType(services.AWSMatcherMemoryDB, "us-east-1", envProdLabels),
			wantDatabases: types.Databases{memorydbDatabaseProd},
		},
		{
			name: "skip unavailable",
			inputClients: &cloud.TestCloudClients{
				MemoryDB: &mocks.MemoryDBMock{
					Clusters:  []*memorydb.Cluster{memorydbProd, memorydbUnavailable},
					TagsByARN: memorydbTagsByARN,
				},
			},
			inputMatchers: makeAWSMatchersForType(services.AWSMatcherMemoryDB, "us-east-1", wildcardLabels),
			wantDatabases: types.Databases{memorydbDatabaseProd},
		},
		{
			name: "skip unsupported",
			inputClients: &cloud.TestCloudClients{
				MemoryDB: &mocks.MemoryDBMock{
					Clusters:  []*memorydb.Cluster{memorydbProd, memorydbUnsupported},
					TagsByARN: memorydbTagsByARN,
				},
			},
			inputMatchers: makeAWSMatchersForType(services.AWSMatcherMemoryDB, "us-east-1", wildcardLabels),
			wantDatabases: types.Databases{memorydbDatabaseProd},
		},
	}
	testAWSFetchers(t, tests...)
}

func makeMemoryDBCluster(t *testing.T, name, region, env string, opts ...func(*memorydb.Cluster)) (*memorydb.Cluster, types.Database, []*memorydb.Tag) {
	cluster := &memorydb.Cluster{
		ARN:        aws.String(fmt.Sprintf("arn:aws:memorydb:%s:123456789012:cluster:%s", region, name)),
		Name:       aws.String(name),
		Status:     aws.String("available"),
		TLSEnabled: aws.Bool(true),
		ClusterEndpoint: &memorydb.Endpoint{
			Address: aws.String("memorydb.localhost"),
			Port:    aws.Int64(6379),
		},
	}

	for _, opt := range opts {
		opt(cluster)
	}

	tags := []*memorydb.Tag{{
		Key:   aws.String("env"),
		Value: aws.String(env),
	}}
	extraLabels := services.ExtraMemoryDBLabels(cluster, tags, nil)

	database, err := services.NewDatabaseFromMemoryDBCluster(cluster, extraLabels)
	require.NoError(t, err)
	common.ApplyAWSDatabaseNameSuffix(database, services.AWSMatcherMemoryDB)
	return cluster, database, tags
}
