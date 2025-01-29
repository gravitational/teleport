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

package cloud

import (
	"context"
	"testing"

	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	memorydbtypes "github.com/aws/aws-sdk-go-v2/service/memorydb/types"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	rsstypes "github.com/aws/aws-sdk-go-v2/service/redshiftserverless/types"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apiawsutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/utils"
)

func TestURLChecker_AWS(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	region := "us-west-2"
	var testCases types.Databases

	// RDS.
	rdsInstance := mocks.RDSInstance("rds-instance", region, nil)
	rdsInstanceDB, err := common.NewDatabaseFromRDSInstance(rdsInstance)
	require.NoError(t, err)
	rdsCluster := mocks.RDSCluster("rds-cluster", region, nil,
		mocks.WithRDSClusterReader,
		mocks.WithRDSClusterCustomEndpoint("my-custom"),
	)
	rdsClusterDBs, err := common.NewDatabasesFromRDSCluster(rdsCluster, []rdstypes.DBInstance{})
	require.NoError(t, err)
	require.Len(t, rdsClusterDBs, 3) // Primary, reader, custom.
	testCases = append(testCases, append(rdsClusterDBs, rdsInstanceDB)...)

	// RDS Proxy.
	rdsProxy := mocks.RDSProxy("rds-proxy", region, "some-vpc")
	rdsProxyDB, err := common.NewDatabaseFromRDSProxy(rdsProxy, nil)
	require.NoError(t, err)
	rdsProxyCustomEndpoint := mocks.RDSProxyCustomEndpoint(rdsProxy, "my-custom", region)
	rdsProxyCustomEndpointDB, err := common.NewDatabaseFromRDSProxyCustomEndpoint(rdsProxy, rdsProxyCustomEndpoint, nil)
	require.NoError(t, err)
	testCases = append(testCases, rdsProxyDB, rdsProxyCustomEndpointDB)

	// Redshift.
	redshiftCluster := mocks.RedshiftCluster("redshift-cluster", region, nil)
	redshiftClusterDB, err := common.NewDatabaseFromRedshiftCluster(&redshiftCluster)
	require.NoError(t, err)
	testCases = append(testCases, redshiftClusterDB)

	// Redshift Serverless.
	redshiftServerlessWorkgroup := mocks.RedshiftServerlessWorkgroup("redshift-serverless", region)
	redshiftServerlessDB, err := common.NewDatabaseFromRedshiftServerlessWorkgroup(redshiftServerlessWorkgroup, nil)
	require.NoError(t, err)
	redshiftServerlessVPCEndpoint := mocks.RedshiftServerlessEndpointAccess(redshiftServerlessWorkgroup, "vpc-endpoint", region)
	redshiftServerlessVPCEndpointDB, err := common.NewDatabaseFromRedshiftServerlessVPCEndpoint(redshiftServerlessVPCEndpoint, redshiftServerlessWorkgroup, nil)
	require.NoError(t, err)
	testCases = append(testCases, redshiftServerlessDB, redshiftServerlessVPCEndpointDB)

	// ElastiCache.
	elastiCacheCluster := mocks.ElastiCacheCluster("elasticache", region, mocks.WithElastiCacheReaderEndpoint)
	elastiCacheClusterDBs, err := common.NewDatabasesFromElastiCacheNodeGroups(elastiCacheCluster, nil)
	require.NoError(t, err)
	require.Len(t, elastiCacheClusterDBs, 2) // Primary, reader.
	elastiCacheClusterConfigurationMode := mocks.ElastiCacheCluster("elasticache-configuration", region, mocks.WithElastiCacheConfigurationEndpoint)
	elastiCacheClusterConfigurationModeDB, err := common.NewDatabaseFromElastiCacheConfigurationEndpoint(elastiCacheClusterConfigurationMode, nil)
	require.NoError(t, err)
	testCases = append(testCases, append(elastiCacheClusterDBs, elastiCacheClusterConfigurationModeDB)...)

	// MemoryDB.
	memoryDBCluster := mocks.MemoryDBCluster("memorydb", region)
	memoryDBClusterDB, err := common.NewDatabaseFromMemoryDBCluster(memoryDBCluster, nil)
	require.NoError(t, err)
	testCases = append(testCases, memoryDBClusterDB)

	// OpenSearch.
	openSearchDomain := mocks.OpenSearchDomain("opensearch", region, mocks.WithOpenSearchCustomEndpoint("custom.com"))
	openSearchDBs, err := common.NewDatabasesFromOpenSearchDomain(openSearchDomain, nil)
	require.NoError(t, err)
	require.Len(t, openSearchDBs, 2) // Primary, custom.
	openSearchVPCDomain := mocks.OpenSearchDomain("opensearch-vpc", region, mocks.WithOpenSearchVPCEndpoint("vpc"))
	openSearchVPCDomainDBs, err := common.NewDatabasesFromOpenSearchDomain(openSearchVPCDomain, nil)
	require.NoError(t, err)
	require.Len(t, openSearchVPCDomainDBs, 1)
	testCases = append(testCases, append(openSearchDBs, openSearchVPCDomainDBs...)...)

	// DocumentDB
	docdbCluster := mocks.DocumentDBCluster("docdb-cluster", region, nil,
		mocks.WithDocumentDBClusterReader,
	)
	docdbClusterDBs, err := common.NewDatabasesFromDocumentDBCluster(docdbCluster)
	require.NoError(t, err)
	require.Len(t, docdbClusterDBs, 2) // Primary, reader.
	testCases = append(testCases, docdbClusterDBs...)

	// Mock cloud clients.
	mockClients := &cloud.TestCloudClients{
		STS: &mocks.STSClientV1{},
	}
	mockClientsUnauth := &cloud.TestCloudClients{
		STS: &mocks.STSClientV1{},
	}

	// Test both check methods.
	// Note that "No permissions" logs should only be printed during the second
	// group ("basic endpoint check").
	methods := []struct {
		name              string
		clients           cloud.Clients
		awsConfigProvider awsconfig.Provider
		awsClients        awsClientProvider
	}{
		{
			name:              "API check",
			clients:           mockClients,
			awsConfigProvider: &mocks.AWSConfigProvider{},
			awsClients: fakeAWSClients{
				ecClient: &mocks.ElastiCacheClient{
					ReplicationGroups: []ectypes.ReplicationGroup{*elastiCacheClusterConfigurationMode, *elastiCacheCluster},
				},
				mdbClient: &mocks.MemoryDBClient{
					Clusters: []memorydbtypes.Cluster{*memoryDBCluster},
				},
				openSearchClient: &mocks.OpenSearchClient{
					Domains: []opensearchtypes.DomainStatus{*openSearchDomain, *openSearchVPCDomain},
				},
				rdsClient: &mocks.RDSClient{
					DBInstances:      []rdstypes.DBInstance{*rdsInstance},
					DBClusters:       []rdstypes.DBCluster{*rdsCluster, *docdbCluster},
					DBProxies:        []rdstypes.DBProxy{*rdsProxy},
					DBProxyEndpoints: []rdstypes.DBProxyEndpoint{*rdsProxyCustomEndpoint},
				},
				redshiftClient: &mocks.RedshiftClient{
					Clusters: []redshifttypes.Cluster{redshiftCluster},
				},
				rssClient: &mocks.RedshiftServerlessClient{
					Workgroups: []rsstypes.Workgroup{*redshiftServerlessWorkgroup},
					Endpoints:  []rsstypes.EndpointAccess{*redshiftServerlessVPCEndpoint},
				},
			},
		},
		{
			name:              "basic endpoint check",
			clients:           mockClientsUnauth,
			awsConfigProvider: &mocks.AWSConfigProvider{},
			awsClients: fakeAWSClients{
				ecClient:         &mocks.ElastiCacheClient{Unauth: true},
				mdbClient:        &mocks.MemoryDBClient{Unauth: true},
				openSearchClient: &mocks.OpenSearchClient{Unauth: true},
				rdsClient:        &mocks.RDSClient{Unauth: true},
				redshiftClient:   &mocks.RedshiftClient{Unauth: true},
				rssClient:        &mocks.RedshiftServerlessClient{Unauth: true},
			},
		},
	}

	for _, method := range methods {
		t.Run(method.name, func(t *testing.T) {
			c := newURLChecker(DiscoveryResourceCheckerConfig{
				Clients:           method.clients,
				AWSConfigProvider: method.awsConfigProvider,
				Logger:            utils.NewSlogLoggerForTests(),
			})
			c.awsClients = method.awsClients

			for _, database := range testCases {
				t.Run(database.GetName(), func(t *testing.T) {
					t.Run("valid", func(t *testing.T) {
						// Special case for OpenSearch custom endpoint where basic endpoint check always fails.
						if database.GetAWS().OpenSearch.EndpointType == apiawsutils.OpenSearchCustomEndpoint &&
							method.name == "basic endpoint check" {
							require.Error(t, c.Check(ctx, database))
							return
						}

						require.NoError(t, c.Check(ctx, database))
					})

					// Make a copy and set an invalid URI.
					t.Run("invalid", func(t *testing.T) {
						invalid := database.Copy()
						invalid.SetURI("localhost:12345")
						require.Error(t, c.Check(ctx, invalid))
					})
				})
			}
		})
	}
}
