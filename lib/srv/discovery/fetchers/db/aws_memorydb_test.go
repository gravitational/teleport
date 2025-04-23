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
	memorydbtypes "github.com/aws/aws-sdk-go-v2/service/memorydb/types"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

func TestMemoryDBFetcher(t *testing.T) {
	t.Parallel()

	memorydbProd, memorydbDatabaseProd, memorydbProdTags := makeMemoryDBCluster(t, "memory1", "us-east-1", "prod", withMemoryDBEngine("valkey"))
	memorydbTest, memorydbDatabaseTest, memorydbTestTags := makeMemoryDBCluster(t, "memory2", "us-east-1", "test")
	memorydbUnavailable, _, memorydbUnavailableTags := makeMemoryDBCluster(t, "memory3", "us-east-1", "prod", func(cluster *memorydbtypes.Cluster) {
		cluster.Status = aws.String("deleting")
	})
	memorydbUnsupported, _, memorydbUnsupportedTags := makeMemoryDBCluster(t, "memory4", "us-east-1", "prod", func(cluster *memorydbtypes.Cluster) {
		cluster.TLSEnabled = aws.Bool(false)
	})
	memorydbTagsByARN := map[string][]memorydbtypes.Tag{
		aws.ToString(memorydbProd.ARN):        memorydbProdTags,
		aws.ToString(memorydbTest.ARN):        memorydbTestTags,
		aws.ToString(memorydbUnavailable.ARN): memorydbUnavailableTags,
		aws.ToString(memorydbUnsupported.ARN): memorydbUnsupportedTags,
	}

	tests := []awsFetcherTest{
		{
			name: "fetch all",
			fetcherCfg: AWSFetcherFactoryConfig{
				AWSClients: fakeAWSClients{
					mdbClient: &mocks.MemoryDBClient{
						Clusters:  []memorydbtypes.Cluster{*memorydbProd, *memorydbTest},
						TagsByARN: memorydbTagsByARN,
					},
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherMemoryDB, "us-east-1", wildcardLabels),
			wantDatabases: types.Databases{memorydbDatabaseProd, memorydbDatabaseTest},
		},
		{
			name: "fetch prod",
			fetcherCfg: AWSFetcherFactoryConfig{
				AWSClients: fakeAWSClients{
					mdbClient: &mocks.MemoryDBClient{
						Clusters:  []memorydbtypes.Cluster{*memorydbProd, *memorydbTest},
						TagsByARN: memorydbTagsByARN,
					},
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherMemoryDB, "us-east-1", envProdLabels),
			wantDatabases: types.Databases{memorydbDatabaseProd},
		},
		{
			name: "skip unavailable",
			fetcherCfg: AWSFetcherFactoryConfig{
				AWSClients: fakeAWSClients{
					mdbClient: &mocks.MemoryDBClient{
						Clusters:  []memorydbtypes.Cluster{*memorydbProd, *memorydbUnavailable},
						TagsByARN: memorydbTagsByARN,
					},
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherMemoryDB, "us-east-1", wildcardLabels),
			wantDatabases: types.Databases{memorydbDatabaseProd},
		},
		{
			name: "skip unsupported",
			fetcherCfg: AWSFetcherFactoryConfig{
				AWSClients: fakeAWSClients{
					mdbClient: &mocks.MemoryDBClient{
						Clusters:  []memorydbtypes.Cluster{*memorydbProd, *memorydbUnsupported},
						TagsByARN: memorydbTagsByARN,
					},
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherMemoryDB, "us-east-1", wildcardLabels),
			wantDatabases: types.Databases{memorydbDatabaseProd},
		},
	}
	testAWSFetchers(t, tests...)
}

func makeMemoryDBCluster(t *testing.T, name, region, env string, opts ...func(*memorydbtypes.Cluster)) (*memorydbtypes.Cluster, types.Database, []memorydbtypes.Tag) {
	cluster := mocks.MemoryDBCluster(name, region, opts...)

	tags := []memorydbtypes.Tag{{
		Key:   aws.String("env"),
		Value: aws.String(env),
	}}
	extraLabels := common.ExtraMemoryDBLabels(cluster, tags, nil)

	database, err := common.NewDatabaseFromMemoryDBCluster(cluster, extraLabels)
	require.NoError(t, err)
	common.ApplyAWSDatabaseNameSuffix(database, types.AWSMatcherMemoryDB)
	return cluster, database, tags
}

func withMemoryDBEngine(engine string) func(*memorydbtypes.Cluster) {
	return func(c *memorydbtypes.Cluster) {
		c.Engine = &engine
	}
}
