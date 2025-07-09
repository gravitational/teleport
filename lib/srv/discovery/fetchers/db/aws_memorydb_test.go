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
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/mocks"
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
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherMemoryDB, "us-east-1", wildcardLabels),
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
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherMemoryDB, "us-east-1", envProdLabels),
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
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherMemoryDB, "us-east-1", wildcardLabels),
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
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherMemoryDB, "us-east-1", wildcardLabels),
			wantDatabases: types.Databases{memorydbDatabaseProd},
		},
	}
	testAWSFetchers(t, tests...)
}

func makeMemoryDBCluster(t *testing.T, name, region, env string, opts ...func(*memorydb.Cluster)) (*memorydb.Cluster, types.Database, []*memorydb.Tag) {
	cluster := mocks.MemoryDBCluster(name, region, opts...)

	tags := []*memorydb.Tag{{
		Key:   aws.String("env"),
		Value: aws.String(env),
	}}
	extraLabels := common.ExtraMemoryDBLabels(cluster, tags, nil)

	database, err := common.NewDatabaseFromMemoryDBCluster(cluster, extraLabels)
	require.NoError(t, err)
	common.ApplyAWSDatabaseNameSuffix(database, types.AWSMatcherMemoryDB)
	return cluster, database, tags
}
