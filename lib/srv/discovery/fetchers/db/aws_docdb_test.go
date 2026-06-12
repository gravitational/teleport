/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

func TestDocumentDBFetcher(t *testing.T) {
	t.Parallel()

	docdbEngine := &rds.DBEngineVersion{
		Engine: aws.String("docdb"),
	}

	clusterProd := mocks.DocumentDBCluster("cluster1", "us-east-1", envProdLabels, mocks.WithDocumentDBClusterReader)
	clusterDev := mocks.DocumentDBCluster("cluster2", "us-east-1", envDevLabels)
	clusterNotAvailable := mocks.DocumentDBCluster("cluster3", "us-east-1", envDevLabels, func(cluster *rds.DBCluster) {
		cluster.Status = aws.String("creating")
	})
	clusterNotSupported := mocks.DocumentDBCluster("cluster4", "us-east-1", envDevLabels, func(cluster *rds.DBCluster) {
		cluster.EngineVersion = aws.String("4.0.0")
	})

	clusterProdDatabases := mustMakeDocumentDBDatabases(t, clusterProd)
	clusterDevDatabases := mustMakeDocumentDBDatabases(t, clusterDev)

	tests := []awsFetcherTest{
		{
			name: "fetch all",
			inputClients: &cloud.TestCloudClients{
				RDS: &mocks.RDSMock{
					DBClusters:       []*rds.DBCluster{clusterProd, clusterDev},
					DBEngineVersions: []*rds.DBEngineVersion{docdbEngine},
				},
			},
			inputMatchers: []types.AWSMatcher{
				{
					Types:   []string{types.AWSMatcherDocumentDB},
					Regions: []string{"us-east-1"},
					Tags:    toTypeLabels(wildcardLabels),
				},
			},
			wantDatabases: append(clusterProdDatabases, clusterDevDatabases...),
		},
		{
			name: "filter by labels",
			inputClients: &cloud.TestCloudClients{
				RDS: &mocks.RDSMock{
					DBClusters:       []*rds.DBCluster{clusterProd, clusterDev},
					DBEngineVersions: []*rds.DBEngineVersion{docdbEngine},
				},
			},
			inputMatchers: []types.AWSMatcher{
				{
					Types:   []string{types.AWSMatcherDocumentDB},
					Regions: []string{"us-east-1"},
					Tags:    toTypeLabels(envProdLabels),
				},
			},
			wantDatabases: clusterProdDatabases,
		},
		{
			name: "skip unsupported databases",
			inputClients: &cloud.TestCloudClients{
				RDS: &mocks.RDSMock{
					DBClusters:       []*rds.DBCluster{clusterProd, clusterNotSupported},
					DBEngineVersions: []*rds.DBEngineVersion{docdbEngine},
				},
			},
			inputMatchers: []types.AWSMatcher{
				{
					Types:   []string{types.AWSMatcherDocumentDB},
					Regions: []string{"us-east-1"},
					Tags:    toTypeLabels(wildcardLabels),
				},
			},
			wantDatabases: clusterProdDatabases,
		},
		{
			name: "skip unavailable databases",
			inputClients: &cloud.TestCloudClients{
				RDS: &mocks.RDSMock{
					DBClusters:       []*rds.DBCluster{clusterProd, clusterNotAvailable},
					DBEngineVersions: []*rds.DBEngineVersion{docdbEngine},
				},
			},
			inputMatchers: []types.AWSMatcher{
				{
					Types:   []string{types.AWSMatcherDocumentDB},
					Regions: []string{"us-east-1"},
					Tags:    toTypeLabels(wildcardLabels),
				},
			},
			wantDatabases: clusterProdDatabases,
		},
	}
	testAWSFetchers(t, tests...)
}

func mustMakeDocumentDBDatabases(t *testing.T, cluster *rds.DBCluster) types.Databases {
	t.Helper()

	databases, err := common.NewDatabasesFromDocumentDBCluster(cluster)
	require.NoError(t, err)

	for _, database := range databases {
		common.ApplyAWSDatabaseNameSuffix(database, types.AWSMatcherDocumentDB)
	}
	return databases
}
