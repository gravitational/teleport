/*
Copyright 2021 Gravitational, Inc.

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

package watchers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/cloud"
	"github.com/gravitational/teleport/lib/srv/db/common"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/aws/aws-sdk-go/service/redshift"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type watchTest struct {
	name              string
	awsMatchers       []services.AWSMatcher
	azureMatchers     []services.AzureMatcher
	clients           common.CloudClients
	expectedDatabases types.Databases
}

// TestWatcher tests cloud databases watcher.
func TestWatcher(t *testing.T) {
	ctx := context.Background()

	rdsInstance1, rdsDatabase1 := makeRDSInstance(t, "instance-1", "us-east-1", map[string]string{"env": "prod"})
	rdsInstance2, _ := makeRDSInstance(t, "instance-2", "us-east-2", map[string]string{"env": "prod"})
	rdsInstance3, _ := makeRDSInstance(t, "instance-3", "us-east-1", map[string]string{"env": "dev"})
	rdsInstance4, rdsDatabase4 := makeRDSInstance(t, "instance-4", "us-west-1", nil)
	rdsInstanceUnavailable, _ := makeRDSInstance(t, "instance-5", "us-west-1", nil, withRDSInstanceStatus("stopped"))
	rdsInstanceUnknownStatus, rdsDatabaseUnknownStatus := makeRDSInstance(t, "instance-5", "us-west-6", nil, withRDSInstanceStatus("status-does-not-exist"))

	auroraCluster1, auroraDatabase1 := makeRDSCluster(t, "cluster-1", "us-east-1", map[string]string{"env": "prod"})
	auroraCluster2, auroraDatabases2 := makeRDSClusterWithExtraEndpoints(t, "cluster-2", "us-east-2", map[string]string{"env": "dev"})
	auroraCluster3, _ := makeRDSCluster(t, "cluster-3", "us-east-2", map[string]string{"env": "prod"})
	auroraClusterUnsupported, _ := makeRDSCluster(t, "serverless", "us-east-1", nil, withRDSClusterEngineMode("serverless"))
	auroraClusterUnavailable, _ := makeRDSCluster(t, "cluster-4", "us-east-1", nil, withRDSClusterStatus("creating"))
	auroraClusterUnknownStatus, auroraDatabaseUnknownStatus := makeRDSCluster(t, "cluster-5", "us-east-1", nil, withRDSClusterStatus("status-does-not-exist"))

	redshiftUse1Prod, redshiftDatabaseUse1Prod := makeRedshiftCluster(t, "us-east-1", "prod")
	redshiftUse1Dev, _ := makeRedshiftCluster(t, "us-east-1", "dev")
	redshiftUse1Unavailable, _ := makeRedshiftCluster(t, "us-east-1", "qa", withRedshiftStatus("paused"))
	redshiftUse1UnknownStatus, redshiftDatabaseUnknownStatus := makeRedshiftCluster(t, "us-east-1", "test", withRedshiftStatus("status-does-not-exist"))

	elasticacheProd, elasticacheDatabaseProd, elasticacheProdTags := makeElastiCacheCluster(t, "ec1", "us-east-1", "prod")
	elasticacheQA, elasticacheDatabaseQA, elasticacheQATags := makeElastiCacheCluster(t, "ec2", "us-east-1", "qa", withElastiCacheConfigurationEndpoint())
	elasticacheTest, _, elasticacheTestTags := makeElastiCacheCluster(t, "ec3", "us-east-1", "test")
	elasticacheUnavailable, _, elasticacheUnavailableTags := makeElastiCacheCluster(t, "ec4", "us-east-1", "prod", func(cluster *elasticache.ReplicationGroup) {
		cluster.Status = aws.String("deleting")
	})
	elasticacheUnsupported, _, elasticacheUnsupportedTags := makeElastiCacheCluster(t, "ec5", "us-east-1", "prod", func(cluster *elasticache.ReplicationGroup) {
		cluster.TransitEncryptionEnabled = aws.Bool(false)
	})
	elasticacheTagsByARN := map[string][]*elasticache.Tag{
		aws.StringValue(elasticacheProd.ARN):        elasticacheProdTags,
		aws.StringValue(elasticacheQA.ARN):          elasticacheQATags,
		aws.StringValue(elasticacheTest.ARN):        elasticacheTestTags,
		aws.StringValue(elasticacheUnavailable.ARN): elasticacheUnavailableTags,
		aws.StringValue(elasticacheUnsupported.ARN): elasticacheUnsupportedTags,
	}

	memorydbProd, memorydbDatabaseProd, memorydbProdTags := makeMemoryDBCluster(t, "memory1", "us-east-1", "prod")
	memorydbTest, _, memorydbTestTags := makeMemoryDBCluster(t, "memory2", "us-east-1", "test")
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

	const (
		eastus        = "eastus"
		eastus2       = "eastus2"
		westus        = "westus"
		subscription1 = "sub1"
		subscription2 = "sub2"
	)
	mySQLServer1, mySQLDatabase1 := makeARMMySQLServer(t, "server-1", subscription1, eastus, map[string]string{"env": "prod"})
	mySQLServer2, _ := makeARMMySQLServer(t, "server-2", subscription1, eastus, map[string]string{"env": "dev"})
	mySQLServer3, _ := makeARMMySQLServer(t, "server-3", subscription1, eastus2, map[string]string{"env": "prod"})
	mySQLServer4, mySQLDatabase4 := makeARMMySQLServer(t, "server-4", subscription2, westus, map[string]string{"env": "prod"})
	mySQLServerUnknownVersion, _ := makeARMMySQLServer(t, "server-5", subscription1, eastus, nil, withARMyMySQLVersion("unknown"))
	mySQLServerUnsupportedVersion, _ := makeARMMySQLServer(t, "server-6", subscription1, eastus, nil, withARMyMySQLVersion(string(armmysql.ServerVersionFive6)))
	mySQLServerDisabledState, _ := makeARMMySQLServer(t, "server-7", subscription1, eastus, nil, withARMMySQLState(string(armmysql.ServerStateDisabled)))
	mySQLServerUnknownState, _ := makeARMMySQLServer(t, "server-8", subscription1, eastus, nil, withARMMySQLState("unknown"))

	postgresServer1, postgresDatabase1 := makeARMPostgresServer(t, "server-1", subscription1, eastus, map[string]string{"env": "prod"})
	postgresServer2, _ := makeARMPostgresServer(t, "server-2", subscription1, eastus, map[string]string{"env": "dev"})
	postgresServer3, _ := makeARMPostgresServer(t, "server-3", subscription1, eastus2, map[string]string{"env": "prod"})
	postgresServer4, postgresDatabase4 := makeARMPostgresServer(t, "server-4", subscription2, westus, map[string]string{"env": "prod"})
	postgresServerUnknownVersion, _ := makeARMPostgresServer(t, "server-5", subscription1, eastus, nil, withARMyPostgresVersion("unknown"))
	postgresServerUnsupportedVersion, _ := makeARMPostgresServer(t, "server-6", subscription1, eastus, nil, withARMyPostgresVersion(""))
	postgresServerDisabledState, _ := makeARMPostgresServer(t, "server-7", subscription1, eastus, nil, withARMPostgresState(string(armpostgresql.ServerStateDisabled)))
	postgresServerUnknownState, _ := makeARMPostgresServer(t, "server-8", subscription1, eastus, nil, withARMPostgresState("unknown"))

	tests := []watchTest{
		{
			name: "RDS labels matching",
			awsMatchers: []services.AWSMatcher{
				{
					Types:   []string{services.AWSMatcherRDS},
					Regions: []string{"us-east-1"},
					Tags:    types.Labels{"env": []string{"prod"}},
				},
				{
					Types:   []string{services.AWSMatcherRDS},
					Regions: []string{"us-east-2"},
					Tags:    types.Labels{"env": []string{"dev"}},
				},
			},
			clients: &common.TestCloudClients{
				RDSPerRegion: map[string]rdsiface.RDSAPI{
					"us-east-1": &cloud.RDSMock{
						DBInstances: []*rds.DBInstance{rdsInstance1, rdsInstance3},
						DBClusters:  []*rds.DBCluster{auroraCluster1},
					},
					"us-east-2": &cloud.RDSMock{
						DBInstances: []*rds.DBInstance{rdsInstance2},
						DBClusters:  []*rds.DBCluster{auroraCluster2, auroraCluster3},
					},
				},
			},
			expectedDatabases: append(types.Databases{rdsDatabase1, auroraDatabase1}, auroraDatabases2...),
		},
		{
			name: "RDS unsupported databases are skipped",
			awsMatchers: []services.AWSMatcher{{
				Types:   []string{services.AWSMatcherRDS},
				Regions: []string{"us-east-1"},
				Tags:    types.Labels{"*": []string{"*"}},
			}},
			clients: &common.TestCloudClients{
				RDSPerRegion: map[string]rdsiface.RDSAPI{
					"us-east-1": &cloud.RDSMock{
						DBClusters: []*rds.DBCluster{auroraCluster1, auroraClusterUnsupported},
					},
				},
			},
			expectedDatabases: types.Databases{auroraDatabase1},
		},
		{
			name: "RDS unavailable databases are skipped",
			awsMatchers: []services.AWSMatcher{{
				Types:   []string{services.AWSMatcherRDS},
				Regions: []string{"us-east-1"},
				Tags:    types.Labels{"*": []string{"*"}},
			}},
			clients: &common.TestCloudClients{
				RDS: &cloud.RDSMock{
					DBInstances: []*rds.DBInstance{rdsInstance1, rdsInstanceUnavailable, rdsInstanceUnknownStatus},
					DBClusters:  []*rds.DBCluster{auroraCluster1, auroraClusterUnavailable, auroraClusterUnknownStatus},
				},
			},
			expectedDatabases: types.Databases{rdsDatabase1, rdsDatabaseUnknownStatus, auroraDatabase1, auroraDatabaseUnknownStatus},
		},
		{
			name: "skip access denied errors",
			awsMatchers: []services.AWSMatcher{{
				Types:   []string{services.AWSMatcherRDS},
				Regions: []string{"ca-central-1", "us-west-1", "us-east-1"},
				Tags:    types.Labels{"*": []string{"*"}},
			}},
			clients: &common.TestCloudClients{
				RDSPerRegion: map[string]rdsiface.RDSAPI{
					"ca-central-1": &cloud.RDSMockUnauth{},
					"us-west-1": &cloud.RDSMockByDBType{
						DBInstances: &cloud.RDSMock{DBInstances: []*rds.DBInstance{rdsInstance4}},
						DBClusters:  &cloud.RDSMockUnauth{},
					},
					"us-east-1": &cloud.RDSMockByDBType{
						DBInstances: &cloud.RDSMockUnauth{},
						DBClusters:  &cloud.RDSMock{DBClusters: []*rds.DBCluster{auroraCluster1}},
					},
				},
			},
			expectedDatabases: types.Databases{rdsDatabase4, auroraDatabase1},
		},
		{
			name: "Redshift labels matching",
			awsMatchers: []services.AWSMatcher{
				{
					Types:   []string{services.AWSMatcherRedshift},
					Regions: []string{"us-east-1"},
					Tags:    types.Labels{"env": []string{"prod"}},
				},
			},
			clients: &common.TestCloudClients{
				Redshift: &cloud.RedshiftMock{
					Clusters: []*redshift.Cluster{redshiftUse1Prod, redshiftUse1Dev},
				},
			},
			expectedDatabases: types.Databases{redshiftDatabaseUse1Prod},
		},
		{
			name: "Redshift unavailable databases are skipped",
			awsMatchers: []services.AWSMatcher{
				{
					Types:   []string{services.AWSMatcherRedshift},
					Regions: []string{"us-east-1"},
					Tags:    types.Labels{"*": []string{"*"}},
				},
			},
			clients: &common.TestCloudClients{
				Redshift: &cloud.RedshiftMock{
					Clusters: []*redshift.Cluster{redshiftUse1Prod, redshiftUse1Unavailable, redshiftUse1UnknownStatus},
				},
			},
			expectedDatabases: types.Databases{redshiftDatabaseUse1Prod, redshiftDatabaseUnknownStatus},
		},
		{
			name: "ElastiCache",
			awsMatchers: []services.AWSMatcher{
				{
					Types:   []string{services.AWSMatcherElastiCache},
					Regions: []string{"us-east-1"},
					Tags:    types.Labels{"env": []string{"prod", "qa"}},
				},
			},
			clients: &common.TestCloudClients{
				ElastiCache: &cloud.ElastiCacheMock{
					ReplicationGroups: []*elasticache.ReplicationGroup{
						elasticacheProd, // labels match
						elasticacheQA,   // labels match
						elasticacheTest, // labels do not match
						elasticacheUnavailable,
						elasticacheUnsupported,
					},
					TagsByARN: elasticacheTagsByARN,
				},
			},
			expectedDatabases: types.Databases{elasticacheDatabaseProd, elasticacheDatabaseQA},
		},
		{
			name: "MemoryDB",
			awsMatchers: []services.AWSMatcher{
				{
					Types:   []string{services.AWSMatcherMemoryDB},
					Regions: []string{"us-east-1"},
					Tags:    types.Labels{"env": []string{"prod"}},
				},
			},
			clients: &common.TestCloudClients{
				MemoryDB: &cloud.MemoryDBMock{
					Clusters: []*memorydb.Cluster{
						memorydbProd, // labels match
						memorydbTest, // labels do not match
						memorydbUnavailable,
						memorydbUnsupported,
					},
					TagsByARN: memorydbTagsByARN,
				},
			},
			expectedDatabases: types.Databases{memorydbDatabaseProd},
		},
		{
			name: "matcher with multiple types",
			awsMatchers: []services.AWSMatcher{
				{
					Types: []string{
						services.AWSMatcherRedshift,
						services.AWSMatcherRDS,
						services.AWSMatcherElastiCache,
						services.AWSMatcherMemoryDB,
					},
					Regions: []string{"us-east-1"},
					Tags:    types.Labels{"env": []string{"prod"}},
				},
			},
			clients: &common.TestCloudClients{
				RDS: &cloud.RDSMock{
					DBClusters: []*rds.DBCluster{auroraCluster1},
				},
				Redshift: &cloud.RedshiftMock{
					Clusters: []*redshift.Cluster{redshiftUse1Prod},
				},
				ElastiCache: &cloud.ElastiCacheMock{
					ReplicationGroups: []*elasticache.ReplicationGroup{elasticacheProd},
					TagsByARN:         elasticacheTagsByARN,
				},
				MemoryDB: &cloud.MemoryDBMock{
					Clusters:  []*memorydb.Cluster{memorydbProd},
					TagsByARN: memorydbTagsByARN,
				},
			},
			expectedDatabases: types.Databases{
				auroraDatabase1,
				redshiftDatabaseUse1Prod,
				elasticacheDatabaseProd,
				memorydbDatabaseProd,
			},
		},
		{
			name: "Azure labels matching",
			azureMatchers: []services.AzureMatcher{
				{
					Subscriptions: []string{subscription1},
					Types:         []string{services.AzureMatcherMySQL, services.AzureMatcherPostgres},
					Regions:       []string{eastus},
					Tags:          types.Labels{"env": []string{"prod"}},
				},
			},
			clients: &common.TestCloudClients{
				AzureMySQLPerSub: map[string]common.AzureMySQLClient{
					subscription1: &cloud.AzureMySQLMock{
						DBServers: []*armmysql.Server{mySQLServer1, mySQLServer2, mySQLServer3},
					},
					subscription2: &cloud.AzureMySQLMock{
						DBServers: []*armmysql.Server{mySQLServer4},
					},
				},
				AzurePostgresPerSub: map[string]common.AzurePostgresClient{
					subscription1: &cloud.AzurePostgresMock{
						DBServers: []*armpostgresql.Server{postgresServer1, postgresServer2, postgresServer3},
					},
					subscription2: &cloud.AzurePostgresMock{
						DBServers: []*armpostgresql.Server{postgresServer4},
					},
				},
			},
			// *server2 tags dont match, *server3 is in eastus2, *server4 is in subscription2
			expectedDatabases: types.Databases{mySQLDatabase1, postgresDatabase1},
		},
		{
			name: "Azure unsupported and unknown database versions are skipped",
			azureMatchers: []services.AzureMatcher{
				{
					Subscriptions: []string{subscription1},
					Types:         []string{services.AzureMatcherMySQL, services.AzureMatcherPostgres},
					Regions:       []string{eastus},
					Tags:          types.Labels{"*": []string{"*"}},
				},
			},
			clients: &common.TestCloudClients{
				AzureMySQL: &cloud.AzureMySQLMock{
					DBServers: []*armmysql.Server{
						mySQLServer1,
						mySQLServerUnknownVersion,
						mySQLServerUnsupportedVersion,
					},
				},
				AzurePostgres: &cloud.AzurePostgresMock{
					DBServers: []*armpostgresql.Server{
						postgresServer1,
						postgresServerUnknownVersion,
						postgresServerUnsupportedVersion,
					},
				},
			},
			expectedDatabases: types.Databases{mySQLDatabase1, postgresDatabase1},
		},
		{
			name: "Azure unavailable databases are skipped",
			azureMatchers: []services.AzureMatcher{
				{
					Subscriptions: []string{subscription1},
					Types:         []string{services.AzureMatcherMySQL, services.AzureMatcherPostgres},
					Regions:       []string{eastus},
					Tags:          types.Labels{"*": []string{"*"}},
				},
			},
			clients: &common.TestCloudClients{
				AzureMySQL: &cloud.AzureMySQLMock{
					DBServers: []*armmysql.Server{
						mySQLServer1,
						mySQLServerDisabledState,
						mySQLServerUnknownState,
					},
				},
				AzurePostgres: &cloud.AzurePostgresMock{
					DBServers: []*armpostgresql.Server{
						postgresServer1,
						postgresServerDisabledState,
						postgresServerUnknownState,
					},
				},
			},
			expectedDatabases: types.Databases{mySQLDatabase1, postgresDatabase1},
		},
		{
			name: "Azure skip access denied errors",
			azureMatchers: []services.AzureMatcher{
				{
					Subscriptions: []string{subscription1, subscription2},
					Types:         []string{services.AzureMatcherMySQL, services.AzureMatcherPostgres},
					Regions:       []string{eastus, westus},
					Tags:          types.Labels{"*": []string{"*"}},
				},
			},
			clients: &common.TestCloudClients{
				AzureMySQLPerSub: map[string]common.AzureMySQLClient{
					subscription1: &cloud.AzureMySQLMockUnauth{
						DBServers: []*armmysql.Server{mySQLServer1, mySQLServer2, mySQLServer3},
					},
					subscription2: &cloud.AzureMySQLMock{
						DBServers: []*armmysql.Server{mySQLServer4},
					},
				},
				AzurePostgresPerSub: map[string]common.AzurePostgresClient{
					subscription1: &cloud.AzurePostgresMockUnauth{
						DBServers: []*armpostgresql.Server{postgresServer1, postgresServer2, postgresServer3},
					},
					subscription2: &cloud.AzurePostgresMock{
						DBServers: []*armpostgresql.Server{postgresServer4},
					},
				},
			},
			expectedDatabases: types.Databases{mySQLDatabase4, postgresDatabase4},
		},
		{
			name: "multiple cloud matchers",
			awsMatchers: []services.AWSMatcher{
				{
					Types: []string{
						services.AWSMatcherRedshift,
						services.AWSMatcherRDS,
						services.AWSMatcherElastiCache,
						services.AWSMatcherMemoryDB,
					},
					Regions: []string{"us-east-1"},
					Tags:    types.Labels{"env": []string{"prod"}},
				},
			},
			azureMatchers: []services.AzureMatcher{
				{
					Subscriptions: []string{subscription1, subscription2},
					Types: []string{
						services.AzureMatcherMySQL,
						services.AzureMatcherPostgres,
					},
					Regions: []string{eastus, westus},
					Tags:    types.Labels{"*": []string{"*"}},
				},
			},
			clients: &common.TestCloudClients{
				RDS: &cloud.RDSMock{
					DBClusters: []*rds.DBCluster{auroraCluster1},
				},
				Redshift: &cloud.RedshiftMock{
					Clusters: []*redshift.Cluster{redshiftUse1Prod},
				},
				ElastiCache: &cloud.ElastiCacheMock{
					ReplicationGroups: []*elasticache.ReplicationGroup{elasticacheProd},
					TagsByARN:         elasticacheTagsByARN,
				},
				MemoryDB: &cloud.MemoryDBMock{
					Clusters:  []*memorydb.Cluster{memorydbProd},
					TagsByARN: memorydbTagsByARN,
				},
				AzureMySQLPerSub: map[string]common.AzureMySQLClient{
					subscription1: &cloud.AzureMySQLMock{
						DBServers: []*armmysql.Server{mySQLServer1},
					},
					subscription2: &cloud.AzureMySQLMock{
						DBServers: []*armmysql.Server{mySQLServer4},
					},
				},
				AzurePostgresPerSub: map[string]common.AzurePostgresClient{
					subscription1: &cloud.AzurePostgresMock{
						DBServers: []*armpostgresql.Server{postgresServer1},
					},
					subscription2: &cloud.AzurePostgresMock{
						DBServers: []*armpostgresql.Server{postgresServer4},
					},
				},
			},
			expectedDatabases: types.Databases{
				auroraDatabase1,
				redshiftDatabaseUse1Prod,
				elasticacheDatabaseProd,
				memorydbDatabaseProd,
				mySQLDatabase1,
				mySQLDatabase4,
				postgresDatabase1,
				postgresDatabase4,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			watcher, err := NewWatcher(ctx,
				WatcherConfig{
					AWSMatchers:   test.awsMatchers,
					AzureMatchers: test.azureMatchers,
					Clients:       test.clients,
				})
			require.NoError(t, err)

			go watcher.fetchAndSend()
			select {
			case databases := <-watcher.DatabasesC():
				// makeFetchers function uses a map for matcher types so
				// databases can come in random orders.
				require.ElementsMatch(t, test.expectedDatabases, databases)
			case <-time.After(time.Second):
				t.Fatal("didn't receive databases after 1 second")
			}
		})
	}
}

func makeARMMySQLServer(t *testing.T, name, subscription, region string, labels map[string]string, opts ...func(*armmysql.Server)) (*armmysql.Server, types.Database) {
	resourceGroup := "defaultRG"
	resourceType := "Microsoft.DBForMySQL/servers"
	id := fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/%v/%v",
		subscription,
		resourceGroup,
		resourceType,
		name,
	)
	// ensure this mock ID is valid and parses
	_, err := arm.ParseResourceID(id)
	require.NoError(t, err)

	fqdn := name + ".mysql" + types.AzureEndpointSuffix
	state := armmysql.ServerStateReady
	version := armmysql.ServerVersionFive7
	server := &armmysql.Server{
		Location: &region,
		Properties: &armmysql.ServerProperties{
			FullyQualifiedDomainName: &fqdn,
			UserVisibleState:         &state,
			Version:                  &version,
		},
		Tags: labelsToAzureTags(labels),
		ID:   &id,
		Name: &name,
		Type: &resourceType,
	}
	for _, opt := range opts {
		opt(server)
	}

	database, err := services.NewDatabaseFromAzureMySQLServer(server)
	require.NoError(t, err)
	return server, database
}

func makeARMPostgresServer(t *testing.T, name, subscription, region string, labels map[string]string, opts ...func(*armpostgresql.Server)) (*armpostgresql.Server, types.Database) {
	resourceGroup := "defaultRG"
	resourceType := "Microsoft.DBForPostgreSQL/servers"
	id := fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/%v/%v",
		subscription,
		resourceGroup,
		resourceType,
		name,
	)
	// ensure this mock ID is valid and parses
	_, err := arm.ParseResourceID(id)
	require.NoError(t, err)

	fqdn := name + ".postgres" + types.AzureEndpointSuffix
	state := armpostgresql.ServerStateReady
	version := armpostgresql.ServerVersionEleven
	server := &armpostgresql.Server{
		Location: &region,
		Properties: &armpostgresql.ServerProperties{
			FullyQualifiedDomainName: &fqdn,
			UserVisibleState:         &state,
			Version:                  &version,
		},
		Tags: labelsToAzureTags(labels),
		ID:   &id,
		Name: &name,
		Type: &resourceType,
	}
	for _, opt := range opts {
		opt(server)
	}

	database, err := services.NewDatabaseFromAzurePostgresServer(server)
	require.NoError(t, err)
	return server, database
}

// withARMMySQLState returns an option function to makeARMMySQLServer to overwrite state.
func withARMMySQLState(state string) func(*armmysql.Server) {
	return func(server *armmysql.Server) {
		state := armmysql.ServerState(state) // ServerState is a type alias for string
		server.Properties.UserVisibleState = &state
	}
}

// withARMyMySQLVersion returns an option function to makeARMMySQLServer to overwrite version.
func withARMyMySQLVersion(version string) func(*armmysql.Server) {
	return func(server *armmysql.Server) {
		version := armmysql.ServerVersion(version) // ServerVersion is a type alias for string
		server.Properties.Version = &version
	}
}

// withARMPostgresState returns an option function to makeARMPostgresServer to overwrite state.
func withARMPostgresState(state string) func(*armpostgresql.Server) {
	return func(server *armpostgresql.Server) {
		state := armpostgresql.ServerState(state) // ServerState is a type alias for string
		server.Properties.UserVisibleState = &state
	}
}

// withARMyPostgresVersion returns an option function to makeARMPostgresServer to overwrite version.
func withARMyPostgresVersion(version string) func(*armpostgresql.Server) {
	return func(server *armpostgresql.Server) {
		version := armpostgresql.ServerVersion(version) // ServerVersion is a type alias for string
		server.Properties.Version = &version
	}
}

func makeRDSInstance(t *testing.T, name, region string, labels map[string]string, opts ...func(*rds.DBInstance)) (*rds.DBInstance, types.Database) {
	instance := &rds.DBInstance{
		DBInstanceArn:        aws.String(fmt.Sprintf("arn:aws:rds:%v:1234567890:db:%v", region, name)),
		DBInstanceIdentifier: aws.String(name),
		DbiResourceId:        aws.String(uuid.New().String()),
		Engine:               aws.String(services.RDSEnginePostgres),
		DBInstanceStatus:     aws.String("available"),
		Endpoint: &rds.Endpoint{
			Address: aws.String("localhost"),
			Port:    aws.Int64(5432),
		},
		TagList: labelsToTags(labels),
	}
	for _, opt := range opts {
		opt(instance)
	}

	database, err := services.NewDatabaseFromRDSInstance(instance)
	require.NoError(t, err)
	return instance, database
}

func makeRDSCluster(t *testing.T, name, region string, labels map[string]string, opts ...func(*rds.DBCluster)) (*rds.DBCluster, types.Database) {
	cluster := &rds.DBCluster{
		DBClusterArn:        aws.String(fmt.Sprintf("arn:aws:rds:%v:1234567890:cluster:%v", region, name)),
		DBClusterIdentifier: aws.String(name),
		DbClusterResourceId: aws.String(uuid.New().String()),
		Engine:              aws.String(services.RDSEngineAuroraMySQL),
		EngineMode:          aws.String(services.RDSEngineModeProvisioned),
		Status:              aws.String("available"),
		Endpoint:            aws.String("localhost"),
		Port:                aws.Int64(3306),
		TagList:             labelsToTags(labels),
	}
	for _, opt := range opts {
		opt(cluster)
	}

	database, err := services.NewDatabaseFromRDSCluster(cluster)
	require.NoError(t, err)
	return cluster, database
}

func makeRedshiftCluster(t *testing.T, region, env string, opts ...func(*redshift.Cluster)) (*redshift.Cluster, types.Database) {
	cluster := &redshift.Cluster{
		ClusterIdentifier:   aws.String(env),
		ClusterNamespaceArn: aws.String(fmt.Sprintf("arn:aws:redshift:%s:1234567890:namespace:%s", region, env)),
		ClusterStatus:       aws.String("available"),
		Endpoint: &redshift.Endpoint{
			Address: aws.String("localhost"),
			Port:    aws.Int64(5439),
		},
		Tags: []*redshift.Tag{{
			Key:   aws.String("env"),
			Value: aws.String(env),
		}},
	}
	for _, opt := range opts {
		opt(cluster)
	}

	database, err := services.NewDatabaseFromRedshiftCluster(cluster)
	require.NoError(t, err)
	return cluster, database
}

func makeRDSClusterWithExtraEndpoints(t *testing.T, name, region string, labels map[string]string) (*rds.DBCluster, types.Databases) {
	cluster := &rds.DBCluster{
		DBClusterArn:        aws.String(fmt.Sprintf("arn:aws:rds:%v:1234567890:cluster:%v", region, name)),
		DBClusterIdentifier: aws.String(name),
		DbClusterResourceId: aws.String(uuid.New().String()),
		Engine:              aws.String(services.RDSEngineAuroraMySQL),
		EngineMode:          aws.String(services.RDSEngineModeProvisioned),
		Status:              aws.String("available"),
		Endpoint:            aws.String("localhost"),
		ReaderEndpoint:      aws.String("reader.host"),
		Port:                aws.Int64(3306),
		TagList:             labelsToTags(labels),
		DBClusterMembers:    []*rds.DBClusterMember{&rds.DBClusterMember{}, &rds.DBClusterMember{}},
		CustomEndpoints: []*string{
			aws.String("custom1.cluster-custom-example.us-east-1.rds.amazonaws.com"),
			aws.String("custom2.cluster-custom-example.us-east-1.rds.amazonaws.com"),
		},
	}

	primaryDatabase, err := services.NewDatabaseFromRDSCluster(cluster)
	require.NoError(t, err)

	readerDatabase, err := services.NewDatabaseFromRDSClusterReaderEndpoint(cluster)
	require.NoError(t, err)

	customDatabases, err := services.NewDatabasesFromRDSClusterCustomEndpoints(cluster)
	require.NoError(t, err)

	return cluster, append(types.Databases{primaryDatabase, readerDatabase}, customDatabases...)
}

func makeElastiCacheCluster(t *testing.T, name, region, env string, opts ...func(*elasticache.ReplicationGroup)) (*elasticache.ReplicationGroup, types.Database, []*elasticache.Tag) {
	cluster := &elasticache.ReplicationGroup{
		ARN:                      aws.String(fmt.Sprintf("arn:aws:elasticache:%s:123456789:replicationgroup:%s", region, name)),
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
		return cluster, database, tags
	}

	databases, err := services.NewDatabasesFromElastiCacheNodeGroups(cluster, extraLabels)
	require.NoError(t, err)
	require.Len(t, databases, 1)
	return cluster, databases[0], tags
}

func makeMemoryDBCluster(t *testing.T, name, region, env string, opts ...func(*memorydb.Cluster)) (*memorydb.Cluster, types.Database, []*memorydb.Tag) {
	cluster := &memorydb.Cluster{
		ARN:        aws.String(fmt.Sprintf("arn:aws:memorydb:%s:123456789:cluster:%s", region, name)),
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
	return cluster, database, tags
}

// withRDSInstanceStatus returns an option function for makeRDSInstance to overwrite status.
func withRDSInstanceStatus(status string) func(*rds.DBInstance) {
	return func(instance *rds.DBInstance) {
		instance.DBInstanceStatus = aws.String(status)
	}
}

// withRDSClusterEngineMode returns an option function for makeRDSCluster to overwrite engine mode.
func withRDSClusterEngineMode(mode string) func(*rds.DBCluster) {
	return func(cluster *rds.DBCluster) {
		cluster.EngineMode = aws.String(mode)
	}
}

// withRDSClusterStatus returns an option function for makeRDSCluster to overwrite status.
func withRDSClusterStatus(status string) func(*rds.DBCluster) {
	return func(cluster *rds.DBCluster) {
		cluster.Status = aws.String(status)
	}
}

// withRedshiftStatus returns an option function for makeRedshiftCluster to overwrite status.
func withRedshiftStatus(status string) func(*redshift.Cluster) {
	return func(cluster *redshift.Cluster) {
		cluster.ClusterStatus = aws.String(status)
	}
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

func labelsToTags(labels map[string]string) (tags []*rds.Tag) {
	for key, val := range labels {
		tags = append(tags, &rds.Tag{
			Key:   aws.String(key),
			Value: aws.String(val),
		})
	}
	return tags
}

func labelsToAzureTags(labels map[string]string) map[string]*string {
	tags := make(map[string]*string, len(labels))
	for k, v := range labels {
		tags[k] = &v
	}
	return tags
}
