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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redis/armredis/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redisenterprise/armredisenterprise"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	azureutils "github.com/gravitational/teleport/api/utils/azure"
	clients "github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/cloud"
)

// TestWatcher tests cloud databases watcher.
func TestWatcher(t *testing.T) {
	ctx := context.Background()

	rdsInstance1, rdsDatabase1 := makeRDSInstance(t, "instance-1", "us-east-1", map[string]string{"env": "prod"})
	rdsInstance2, _ := makeRDSInstance(t, "instance-2", "us-east-2", map[string]string{"env": "prod"})
	rdsInstance3, _ := makeRDSInstance(t, "instance-3", "us-east-1", map[string]string{"env": "dev"})
	rdsInstance4, rdsDatabase4 := makeRDSInstance(t, "instance-4", "us-west-1", nil)
	rdsInstance5, rdsDatabase5 := makeRDSInstance(t, "instance-5", "us-east-2", map[string]string{"env": "dev"})
	rdsInstanceUnavailable, _ := makeRDSInstance(t, "instance-5", "us-west-1", nil, withRDSInstanceStatus("stopped"))
	rdsInstanceUnknownStatus, rdsDatabaseUnknownStatus := makeRDSInstance(t, "instance-5", "us-west-6", nil, withRDSInstanceStatus("status-does-not-exist"))
	auroraMySQLEngine := &rds.DBEngineVersion{Engine: aws.String(services.RDSEngineAuroraMySQL)}
	postgresEngine := &rds.DBEngineVersion{Engine: aws.String(services.RDSEnginePostgres)}

	auroraCluster1, auroraDatabase1 := makeRDSCluster(t, "cluster-1", "us-east-1", map[string]string{"env": "prod"})
	auroraCluster2, auroraDatabases2 := makeRDSClusterWithExtraEndpoints(t, "cluster-2", "us-east-2", map[string]string{"env": "dev"}, true)
	auroraCluster3, _ := makeRDSCluster(t, "cluster-3", "us-east-2", map[string]string{"env": "prod"})
	auroraClusterUnsupported, _ := makeRDSCluster(t, "serverless", "us-east-1", nil, withRDSClusterEngineMode("serverless"))
	auroraClusterUnavailable, _ := makeRDSCluster(t, "cluster-4", "us-east-1", nil, withRDSClusterStatus("creating"))
	auroraClusterUnknownStatus, auroraDatabaseUnknownStatus := makeRDSCluster(t, "cluster-5", "us-east-1", nil, withRDSClusterStatus("status-does-not-exist"))
	auroraClusterNoWriter, auroraDatabasesNoWriter := makeRDSClusterWithExtraEndpoints(t, "cluster-6", "us-east-1", map[string]string{"env": "dev"}, false)

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

	rdsProxyVpc1, rdsProxyDatabaseVpc1 := makeRDSProxy(t, "rds-proxy-1", "us-east-1", "vpc1")
	rdsProxyVpc2, _ := makeRDSProxy(t, "rds-proxy-2", "us-east-1", "vpc2")
	rdsProxyEndpointVpc1, rdsProxyEndpointDatabaseVpc1 := makeRDSProxyCustomEndpoint(t, rdsProxyVpc1, "endpoint-1", "us-east-1")
	rdsProxyEndpointVpc2, _ := makeRDSProxyCustomEndpoint(t, rdsProxyVpc2, "endpoint-2", "us-east-1")

	const (
		group1        = "group1"
		group2        = "group2"
		eastus        = "eastus"
		eastus2       = "eastus2"
		westus        = "westus"
		subscription1 = "sub1"
		subscription2 = "sub2"
	)

	azureSub1 := makeAzureSubscription(t, subscription1)
	azureSub2 := makeAzureSubscription(t, subscription2)

	azMySQLServer1, azMySQLDB1 := makeAzureMySQLServer(t, "server-1", subscription1, group1, eastus, map[string]string{"env": "prod"})
	azMySQLServer2, _ := makeAzureMySQLServer(t, "server-2", subscription1, group1, eastus, map[string]string{"env": "dev"})
	azMySQLServer3, _ := makeAzureMySQLServer(t, "server-3", subscription1, group1, eastus2, map[string]string{"env": "prod"})
	azMySQLServer4, azMySQLDB4 := makeAzureMySQLServer(t, "server-4", subscription2, group1, westus, map[string]string{"env": "prod"})
	azMySQLServer5, _ := makeAzureMySQLServer(t, "server-5", subscription1, group2, eastus, map[string]string{"env": "prod"})
	azMySQLServerUnknownVersion, azMySQLDBUnknownVersion := makeAzureMySQLServer(t, "server-6", subscription1, group1, eastus, nil, withAzureMySQLVersion("unknown"))
	azMySQLServerUnsupportedVersion, _ := makeAzureMySQLServer(t, "server-7", subscription1, group1, eastus, nil, withAzureMySQLVersion(string(armmysql.ServerVersionFive6)))
	azMySQLServerDisabledState, _ := makeAzureMySQLServer(t, "server-8", subscription1, group1, eastus, nil, withAzureMySQLState(string(armmysql.ServerStateDisabled)))
	azMySQLServerUnknownState, azMySQLDBUnknownState := makeAzureMySQLServer(t, "server-9", subscription1, group1, eastus, nil, withAzureMySQLState("unknown"))

	azPostgresServer1, azPostgresDB1 := makeAzurePostgresServer(t, "server-1", subscription1, group1, eastus, map[string]string{"env": "prod"})
	azPostgresServer2, _ := makeAzurePostgresServer(t, "server-2", subscription1, group1, eastus, map[string]string{"env": "dev"})
	azPostgresServer3, _ := makeAzurePostgresServer(t, "server-3", subscription1, group1, eastus2, map[string]string{"env": "prod"})
	azPostgresServer4, azPostgresDB4 := makeAzurePostgresServer(t, "server-4", subscription2, group1, westus, map[string]string{"env": "prod"})
	azPostgresServer5, _ := makeAzurePostgresServer(t, "server-5", subscription1, group2, eastus, map[string]string{"env": "prod"})
	azPostgresServerUnknownVersion, azPostgresDBUnknownVersion := makeAzurePostgresServer(t, "server-6", subscription1, group1, eastus, nil, withAzurePostgresVersion("unknown"))
	azPostgresServerDisabledState, _ := makeAzurePostgresServer(t, "server-8", subscription1, group1, eastus, nil, withAzurePostgresState(string(armpostgresql.ServerStateDisabled)))
	azPostgresServerUnknownState, azPostgresDBUnknownState := makeAzurePostgresServer(t, "server-9", subscription1, group1, eastus, nil, withAzurePostgresState("unknown"))

	// Note that the Azure Redis APIs may return location in their display
	// names (eg. "East US"). The Azure fetcher should normalize location names
	// so region matcher "eastus" will match "East US".
	azRedisServer, azRedisDB := makeAzureRedisServer(t, "redis", subscription1, group1, "East US", map[string]string{"env": "prod"})
	azRedisEnterpriseCluster, azRedisEnterpriseDatabase, azRedisEnterpriseDB := makeAzureRedisEnterpriseCluster(t, "redis-enterprise", subscription1, group1, eastus, map[string]string{"env": "prod"})

	tests := []struct {
		name              string
		awsMatchers       []services.AWSMatcher
		azureMatchers     []services.AzureMatcher
		clients           clients.Clients
		expectedDatabases types.Databases
	}{
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
			clients: &clients.TestCloudClients{
				RDSPerRegion: map[string]rdsiface.RDSAPI{
					"us-east-1": &cloud.RDSMock{
						DBInstances:      []*rds.DBInstance{rdsInstance1, rdsInstance3},
						DBClusters:       []*rds.DBCluster{auroraCluster1},
						DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine, postgresEngine},
					},
					"us-east-2": &cloud.RDSMock{
						DBInstances:      []*rds.DBInstance{rdsInstance2},
						DBClusters:       []*rds.DBCluster{auroraCluster2, auroraCluster3},
						DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine, postgresEngine},
					},
				},
			},
			expectedDatabases: append(types.Databases{rdsDatabase1, auroraDatabase1}, auroraDatabases2...),
		},
		{
			name: "RDS unrecognized engines are skipped",
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
			clients: &clients.TestCloudClients{
				RDSPerRegion: map[string]rdsiface.RDSAPI{
					"us-east-1": &cloud.RDSMock{
						DBInstances:      []*rds.DBInstance{rdsInstance1, rdsInstance3},
						DBClusters:       []*rds.DBCluster{auroraCluster1},
						DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine},
					},
					"us-east-2": &cloud.RDSMock{
						DBInstances:      []*rds.DBInstance{rdsInstance5},
						DBClusters:       []*rds.DBCluster{auroraCluster2, auroraCluster3},
						DBEngineVersions: []*rds.DBEngineVersion{postgresEngine},
					},
				},
			},
			expectedDatabases: types.Databases{auroraDatabase1, rdsDatabase5},
		},
		{
			name: "RDS unsupported databases are skipped",
			awsMatchers: []services.AWSMatcher{{
				Types:   []string{services.AWSMatcherRDS},
				Regions: []string{"us-east-1"},
				Tags:    types.Labels{"*": []string{"*"}},
			}},
			clients: &clients.TestCloudClients{
				RDSPerRegion: map[string]rdsiface.RDSAPI{
					"us-east-1": &cloud.RDSMock{
						DBClusters:       []*rds.DBCluster{auroraCluster1, auroraClusterUnsupported},
						DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine},
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
			clients: &clients.TestCloudClients{
				RDS: &cloud.RDSMock{
					DBInstances:      []*rds.DBInstance{rdsInstance1, rdsInstanceUnavailable, rdsInstanceUnknownStatus},
					DBClusters:       []*rds.DBCluster{auroraCluster1, auroraClusterUnavailable, auroraClusterUnknownStatus},
					DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine, postgresEngine},
				},
			},
			expectedDatabases: types.Databases{rdsDatabase1, rdsDatabaseUnknownStatus, auroraDatabase1, auroraDatabaseUnknownStatus},
		},
		{
			name: "RDS Aurora cluster no writer",
			awsMatchers: []services.AWSMatcher{{
				Types:   []string{services.AWSMatcherRDS},
				Regions: []string{"us-east-1"},
				Tags:    types.Labels{"*": []string{"*"}},
			}},
			clients: &clients.TestCloudClients{
				RDS: &cloud.RDSMock{
					DBClusters:       []*rds.DBCluster{auroraClusterNoWriter},
					DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine},
				},
			},
			expectedDatabases: auroraDatabasesNoWriter,
		},
		{
			name: "skip access denied errors",
			awsMatchers: []services.AWSMatcher{{
				Types:   []string{services.AWSMatcherRDS},
				Regions: []string{"ca-central-1", "us-west-1", "us-east-1"},
				Tags:    types.Labels{"*": []string{"*"}},
			}},
			clients: &clients.TestCloudClients{
				RDSPerRegion: map[string]rdsiface.RDSAPI{
					"ca-central-1": &cloud.RDSMockUnauth{},
					"us-west-1": &cloud.RDSMockByDBType{
						DBInstances: &cloud.RDSMock{
							DBInstances:      []*rds.DBInstance{rdsInstance4},
							DBEngineVersions: []*rds.DBEngineVersion{postgresEngine},
						},
						DBClusters: &cloud.RDSMockUnauth{},
						DBProxies:  &cloud.RDSMockUnauth{},
					},
					"us-east-1": &cloud.RDSMockByDBType{
						DBInstances: &cloud.RDSMockUnauth{},
						DBClusters: &cloud.RDSMock{
							DBClusters:       []*rds.DBCluster{auroraCluster1},
							DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine},
						},
						DBProxies: &cloud.RDSMockUnauth{},
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
			clients: &clients.TestCloudClients{
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
			clients: &clients.TestCloudClients{
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
			clients: &clients.TestCloudClients{
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
			clients: &clients.TestCloudClients{
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
			name: "RDS Proxy",
			awsMatchers: []services.AWSMatcher{{
				Types:   []string{services.AWSMatcherRDSProxy},
				Regions: []string{"us-east-1"},
				Tags:    types.Labels{"vpc-id": []string{"vpc1"}},
			}},
			clients: &clients.TestCloudClients{
				RDS: &cloud.RDSMock{
					DBProxies:         []*rds.DBProxy{rdsProxyVpc1, rdsProxyVpc2},
					DBProxyEndpoints:  []*rds.DBProxyEndpoint{rdsProxyEndpointVpc1, rdsProxyEndpointVpc2},
					DBProxyTargetPort: 9999,
				},
			},
			expectedDatabases: types.Databases{rdsProxyDatabaseVpc1, rdsProxyEndpointDatabaseVpc1},
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
			clients: &clients.TestCloudClients{
				RDS: &cloud.RDSMock{
					DBClusters:       []*rds.DBCluster{auroraCluster1},
					DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine},
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
					Subscriptions:  []string{subscription1},
					ResourceGroups: []string{group1},
					Types:          []string{services.AzureMatcherMySQL, services.AzureMatcherPostgres},
					Regions:        []string{eastus},
					ResourceTags:   types.Labels{"env": []string{"prod"}},
				},
			},
			clients: &clients.TestCloudClients{
				AzureMySQLPerSub: map[string]azure.DBServersClient{
					subscription1: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
						DBServers: []*armmysql.Server{azMySQLServer1, azMySQLServer2, azMySQLServer3, azMySQLServer5},
					}),
					subscription2: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
						DBServers: []*armmysql.Server{azMySQLServer4},
					}),
				},
				AzurePostgresPerSub: map[string]azure.DBServersClient{
					subscription1: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
						DBServers: []*armpostgresql.Server{azPostgresServer1, azPostgresServer2, azPostgresServer3, azPostgresServer5},
					}),
					subscription2: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
						DBServers: []*armpostgresql.Server{azPostgresServer4},
					}),
				},
			},
			// server2 tags don't match, server3 is in eastus2, server4 is in subscription2, server5 is in group2
			expectedDatabases: types.Databases{azMySQLDB1, azPostgresDB1},
		},
		{
			name: "Azure matching labels with all subscriptions, resource groups, and regions",
			azureMatchers: []services.AzureMatcher{
				{
					Subscriptions:  []string{"*"},
					ResourceGroups: []string{"*"},
					Types:          []string{services.AzureMatcherMySQL, services.AzureMatcherPostgres},
					Regions:        []string{"*"},
					ResourceTags:   types.Labels{"env": []string{"prod"}},
				},
			},
			clients: &clients.TestCloudClients{
				AzureMySQLPerSub: map[string]azure.DBServersClient{
					subscription1: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
						DBServers: []*armmysql.Server{azMySQLServer1},
					}),
					subscription2: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
						DBServers: []*armmysql.Server{azMySQLServer4},
					}),
				},
				AzurePostgresPerSub: map[string]azure.DBServersClient{
					subscription1: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
						DBServers: []*armpostgresql.Server{azPostgresServer1},
					}),
					subscription2: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
						DBServers: []*armpostgresql.Server{azPostgresServer4},
					}),
				},
				AzureSubscriptionClient: azure.NewSubscriptionClient(&azure.ARMSubscriptionsMock{
					Subscriptions: []*armsubscription.Subscription{azureSub1, azureSub2},
				}),
			},
			expectedDatabases: types.Databases{azMySQLDB1, azMySQLDB4, azPostgresDB1, azPostgresDB4},
		},
		{
			name: "Azure unsupported and unknown database versions are skipped",
			azureMatchers: []services.AzureMatcher{
				{
					Subscriptions:  []string{subscription1},
					ResourceGroups: []string{"*"},
					Types:          []string{services.AzureMatcherMySQL, services.AzureMatcherPostgres},
					Regions:        []string{eastus},
					ResourceTags:   types.Labels{"*": []string{"*"}},
				},
			},
			clients: &clients.TestCloudClients{
				AzureMySQL: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
					DBServers: []*armmysql.Server{
						azMySQLServer1,
						azMySQLServerUnknownVersion,
						azMySQLServerUnsupportedVersion,
					},
				}),
				AzurePostgres: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
					DBServers: []*armpostgresql.Server{
						azPostgresServer1,
						azPostgresServerUnknownVersion,
					},
				}),
			},
			expectedDatabases: types.Databases{azMySQLDB1, azMySQLDBUnknownVersion, azPostgresDB1, azPostgresDBUnknownVersion},
		},
		{
			name: "Azure unavailable databases are skipped",
			azureMatchers: []services.AzureMatcher{
				{
					Subscriptions:  []string{subscription1},
					ResourceGroups: []string{"*"},
					Types:          []string{services.AzureMatcherMySQL, services.AzureMatcherPostgres},
					Regions:        []string{eastus},
					ResourceTags:   types.Labels{"*": []string{"*"}},
				},
			},
			clients: &clients.TestCloudClients{
				AzureMySQL: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
					DBServers: []*armmysql.Server{
						azMySQLServer1,
						azMySQLServerDisabledState,
						azMySQLServerUnknownState,
					},
				}),
				AzurePostgres: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
					DBServers: []*armpostgresql.Server{
						azPostgresServer1,
						azPostgresServerDisabledState,
						azPostgresServerUnknownState,
					},
				}),
			},
			expectedDatabases: types.Databases{azMySQLDB1, azMySQLDBUnknownState, azPostgresDB1, azPostgresDBUnknownState},
		},
		{
			name: "Azure skip access denied errors",
			azureMatchers: []services.AzureMatcher{
				{
					Subscriptions:  []string{subscription1, subscription2},
					ResourceGroups: []string{"*"},
					Types:          []string{services.AzureMatcherMySQL, services.AzureMatcherPostgres},
					Regions:        []string{eastus, westus},
					ResourceTags:   types.Labels{"*": []string{"*"}},
				},
			},
			clients: &clients.TestCloudClients{
				AzureMySQLPerSub: map[string]azure.DBServersClient{
					subscription1: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
						DBServers: []*armmysql.Server{azMySQLServer1},
						NoAuth:    true,
					}),
					subscription2: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
						DBServers: []*armmysql.Server{azMySQLServer4},
					}),
				},
				AzurePostgresPerSub: map[string]azure.DBServersClient{
					subscription1: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
						DBServers: []*armpostgresql.Server{azPostgresServer1},
						NoAuth:    true,
					}),
					subscription2: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
						DBServers: []*armpostgresql.Server{azPostgresServer4},
					}),
				},
			},
			expectedDatabases: types.Databases{azMySQLDB4, azPostgresDB4},
		},
		{
			name: "Azure skip group not found errors",
			azureMatchers: []services.AzureMatcher{
				{
					Subscriptions:  []string{subscription1},
					ResourceGroups: []string{"foobar", group1, "baz"},
					Types:          []string{services.AzureMatcherMySQL, services.AzureMatcherPostgres},
					Regions:        []string{eastus, westus},
					ResourceTags:   types.Labels{"*": []string{"*"}},
				},
			},
			clients: &clients.TestCloudClients{
				AzureMySQL: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
					DBServers: []*armmysql.Server{
						azMySQLServer1,
					},
				}),
				AzurePostgres: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
					DBServers: []*armpostgresql.Server{
						azPostgresServer1,
					},
				}),
			},
			expectedDatabases: types.Databases{azMySQLDB1, azPostgresDB1},
		},
		{
			name: "Azure Redis",
			azureMatchers: []services.AzureMatcher{
				{
					Types:        []string{services.AzureMatcherRedis},
					ResourceTags: types.Labels{"env": []string{"prod"}},
					Regions:      []string{eastus},
				},
			},
			clients: &clients.TestCloudClients{
				AzureSubscriptionClient: azure.NewSubscriptionClient(&azure.ARMSubscriptionsMock{
					Subscriptions: []*armsubscription.Subscription{azureSub1},
				}),
				AzureRedis: azure.NewRedisClientByAPI(&azure.ARMRedisMock{
					Servers: []*armredis.ResourceInfo{azRedisServer},
				}),
				AzureRedisEnterprise: azure.NewRedisEnterpriseClientByAPI(
					&azure.ARMRedisEnterpriseClusterMock{
						Clusters: []*armredisenterprise.Cluster{azRedisEnterpriseCluster},
					},
					&azure.ARMRedisEnterpriseDatabaseMock{
						Databases: []*armredisenterprise.Database{azRedisEnterpriseDatabase},
					},
				),
			},
			expectedDatabases: types.Databases{azRedisDB, azRedisEnterpriseDB},
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
					Subscriptions:  []string{subscription1, subscription2},
					ResourceGroups: []string{"group1"},
					Types: []string{
						services.AzureMatcherMySQL,
						services.AzureMatcherPostgres,
					},
					Regions:      []string{eastus, westus},
					ResourceTags: types.Labels{"*": []string{"*"}},
				},
			},
			clients: &clients.TestCloudClients{
				RDS: &cloud.RDSMock{
					DBClusters:       []*rds.DBCluster{auroraCluster1},
					DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine},
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
				AzureMySQLPerSub: map[string]azure.DBServersClient{
					subscription1: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
						DBServers: []*armmysql.Server{azMySQLServer1},
					}),
					subscription2: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
						DBServers: []*armmysql.Server{azMySQLServer4},
					}),
				},
				AzurePostgresPerSub: map[string]azure.DBServersClient{
					subscription1: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
						DBServers: []*armpostgresql.Server{azPostgresServer1},
					}),
					subscription2: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
						DBServers: []*armpostgresql.Server{azPostgresServer4},
					}),
				},
			},
			expectedDatabases: types.Databases{
				auroraDatabase1,
				redshiftDatabaseUse1Prod,
				elasticacheDatabaseProd,
				memorydbDatabaseProd,
				azMySQLDB1,
				azMySQLDB4,
				azPostgresDB1,
				azPostgresDB4,
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

			checkFetchAndSend(t, watcher, test.expectedDatabases)
		})
	}

	// Test that newly added Azure subscriptions are discovered by "*" subscription fetchers
	t.Run("Azure subscription discovery", func(t *testing.T) {
		mockSubscriptions := &azure.ARMSubscriptionsMock{
			Subscriptions: []*armsubscription.Subscription{azureSub1},
		}
		watcher, err := NewWatcher(ctx,
			WatcherConfig{
				Clients: &clients.TestCloudClients{
					AzureMySQLPerSub: map[string]azure.DBServersClient{
						subscription1: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
							DBServers: []*armmysql.Server{azMySQLServer1},
						}),
						subscription2: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
							DBServers: []*armmysql.Server{azMySQLServer4},
						}),
					},
					AzurePostgresPerSub: map[string]azure.DBServersClient{
						subscription1: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
							DBServers: []*armpostgresql.Server{azPostgresServer1},
						}),
						subscription2: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
							DBServers: []*armpostgresql.Server{azPostgresServer4},
						}),
					},
					AzureSubscriptionClient: azure.NewSubscriptionClient(mockSubscriptions),
				},
				AzureMatchers: []services.AzureMatcher{
					{
						Subscriptions:  []string{"*"},
						ResourceGroups: []string{"*"},
						Types:          []string{services.AzureMatcherMySQL, services.AzureMatcherPostgres},
						Regions:        []string{"*"},
						ResourceTags:   types.Labels{"*": []string{"*"}},
					},
				},
			})
		require.NoError(t, err)

		// subscription API mock should return just databases in subscription1
		expectedDatabases := types.Databases{azMySQLDB1, azPostgresDB1}
		checkFetchAndSend(t, watcher, expectedDatabases)

		// Mock adding a new subscription
		mockSubscriptions.Subscriptions = []*armsubscription.Subscription{azureSub1, azureSub2}
		// Update expectation to include databases from newly added subscription2
		expectedDatabases = types.Databases{azMySQLDB1, azMySQLDB4, azPostgresDB1, azPostgresDB4}
		checkFetchAndSend(t, watcher, expectedDatabases)
	})
}

// checkFetchAndSend checks that the watcher fetches and sends the expected databases
func checkFetchAndSend(t *testing.T, watcher *Watcher, expectedDatabases types.Databases) {
	go watcher.fetchAndSend()
	select {
	case databases := <-watcher.DatabasesC():
		// makeFetchers function uses a map for matcher types so
		// databases can come in random orders.
		require.ElementsMatch(t, expectedDatabases, databases)
	case <-time.After(time.Second):
		t.Fatal("didn't receive databases after 1 second")
	}
}

func makeAzureMySQLServer(t *testing.T, name, subscription, group, region string, labels map[string]string, opts ...func(*armmysql.Server)) (*armmysql.Server, types.Database) {
	resourceType := "Microsoft.DBforMySQL/servers"
	id := fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/%v/%v",
		subscription,
		group,
		resourceType,
		name,
	)

	fqdn := name + ".mysql" + azureutils.DatabaseEndpointSuffix
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

	azureDBServer := azure.ServerFromMySQLServer(server)

	database, err := services.NewDatabaseFromAzureServer(azureDBServer)
	require.NoError(t, err)
	return server, database
}

func makeAzureSubscription(t *testing.T, subID string) *armsubscription.Subscription {
	return &armsubscription.Subscription{
		SubscriptionID: &subID,
		State:          to.Ptr(armsubscription.SubscriptionStateEnabled),
	}
}

func makeAzurePostgresServer(t *testing.T, name, subscription, group, region string, labels map[string]string, opts ...func(*armpostgresql.Server)) (*armpostgresql.Server, types.Database) {
	resourceType := "Microsoft.DBforPostgreSQL/servers"
	id := fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/%v/%v",
		subscription,
		group,
		resourceType,
		name,
	)

	fqdn := name + ".postgres" + azureutils.DatabaseEndpointSuffix
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

	azureDBServer := azure.ServerFromPostgresServer(server)

	database, err := services.NewDatabaseFromAzureServer(azureDBServer)
	require.NoError(t, err)
	return server, database
}

func makeAzureRedisServer(t *testing.T, name, subscription, group, region string, labels map[string]string) (*armredis.ResourceInfo, types.Database) {
	resourceInfo := &armredis.ResourceInfo{
		Name:     to.Ptr(name),
		ID:       to.Ptr(fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/Microsoft.Cache/Redis/%v", subscription, group, name)),
		Location: to.Ptr(region),
		Tags:     labelsToAzureTags(labels),
		Properties: &armredis.Properties{
			HostName:          to.Ptr(fmt.Sprintf("%v.redis.cache.windows.net", name)),
			SSLPort:           to.Ptr(int32(6380)),
			ProvisioningState: to.Ptr(armredis.ProvisioningStateSucceeded),
		},
	}

	database, err := services.NewDatabaseFromAzureRedis(resourceInfo)
	require.NoError(t, err)
	return resourceInfo, database
}

func makeAzureRedisEnterpriseCluster(t *testing.T, cluster, subscription, group, region string, labels map[string]string) (*armredisenterprise.Cluster, *armredisenterprise.Database, types.Database) {
	armCluster := &armredisenterprise.Cluster{
		Name:     to.Ptr(cluster),
		ID:       to.Ptr(fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/Microsoft.Cache/redisEnterprise/%v", subscription, group, cluster)),
		Location: to.Ptr(region),
		Tags:     labelsToAzureTags(labels),
		Properties: &armredisenterprise.ClusterProperties{
			HostName: to.Ptr(fmt.Sprintf("%v.%v.redisenterprise.cache.azure.net", cluster, region)),
		},
	}
	armDatabase := &armredisenterprise.Database{
		Name: to.Ptr("default"),
		ID:   to.Ptr(fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/Microsoft.Cache/redisEnterprise/%v/databases/default", subscription, group, cluster)),
		Properties: &armredisenterprise.DatabaseProperties{
			ProvisioningState: to.Ptr(armredisenterprise.ProvisioningStateSucceeded),
			Port:              to.Ptr(int32(10000)),
			ClusteringPolicy:  to.Ptr(armredisenterprise.ClusteringPolicyOSSCluster),
			ClientProtocol:    to.Ptr(armredisenterprise.ProtocolEncrypted),
		},
	}

	database, err := services.NewDatabaseFromAzureRedisEnterprise(armCluster, armDatabase)
	require.NoError(t, err)
	return armCluster, armDatabase, database
}

// withAzureMySQLState returns an option function to makeARMMySQLServer to overwrite state.
func withAzureMySQLState(state string) func(*armmysql.Server) {
	return func(server *armmysql.Server) {
		state := armmysql.ServerState(state) // ServerState is a type alias for string
		server.Properties.UserVisibleState = &state
	}
}

// withAzureMySQLVersion returns an option function to makeARMMySQLServer to overwrite version.
func withAzureMySQLVersion(version string) func(*armmysql.Server) {
	return func(server *armmysql.Server) {
		version := armmysql.ServerVersion(version) // ServerVersion is a type alias for string
		server.Properties.Version = &version
	}
}

// withAzurePostgresState returns an option function to makeARMPostgresServer to overwrite state.
func withAzurePostgresState(state string) func(*armpostgresql.Server) {
	return func(server *armpostgresql.Server) {
		state := armpostgresql.ServerState(state) // ServerState is a type alias for string
		server.Properties.UserVisibleState = &state
	}
}

// withAzurePostgresVersion returns an option function to makeARMPostgresServer to overwrite version.
func withAzurePostgresVersion(version string) func(*armpostgresql.Server) {
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
		DBClusterMembers: []*rds.DBClusterMember{&rds.DBClusterMember{
			IsClusterWriter: aws.Bool(true), // Only one writer.
		}},
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

func makeRDSClusterWithExtraEndpoints(t *testing.T, name, region string, labels map[string]string, hasWriter bool) (*rds.DBCluster, types.Databases) {
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
		DBClusterMembers: []*rds.DBClusterMember{&rds.DBClusterMember{
			IsClusterWriter: aws.Bool(false), // Add reader by default. Writer is added below based on hasWriter.
		}},
		CustomEndpoints: []*string{
			aws.String("custom1.cluster-custom-example.us-east-1.rds.amazonaws.com"),
			aws.String("custom2.cluster-custom-example.us-east-1.rds.amazonaws.com"),
		},
	}

	var databases types.Databases

	if hasWriter {
		cluster.DBClusterMembers = append(cluster.DBClusterMembers, &rds.DBClusterMember{
			IsClusterWriter: aws.Bool(true), // Add writer.
		})

		primaryDatabase, err := services.NewDatabaseFromRDSCluster(cluster)
		require.NoError(t, err)
		databases = append(databases, primaryDatabase)
	}

	readerDatabase, err := services.NewDatabaseFromRDSClusterReaderEndpoint(cluster)
	require.NoError(t, err)
	databases = append(databases, readerDatabase)

	customDatabases, err := services.NewDatabasesFromRDSClusterCustomEndpoints(cluster)
	require.NoError(t, err)
	databases = append(databases, customDatabases...)

	return cluster, databases
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

func makeRDSProxy(t *testing.T, name, region, vpcID string) (*rds.DBProxy, types.Database) {
	rdsProxy := &rds.DBProxy{
		DBProxyArn:   aws.String(fmt.Sprintf("arn:aws:rds:%s:1234567890:db-proxy:prx-%s", region, name)),
		DBProxyName:  aws.String(name),
		EngineFamily: aws.String(rds.EngineFamilyMysql),
		Endpoint:     aws.String("localhost"),
		VpcId:        aws.String(vpcID),
		RequireTLS:   aws.Bool(true),
		Status:       aws.String("available"),
	}

	rdsProxyDatabase, err := services.NewDatabaseFromRDSProxy(rdsProxy, 9999, nil)
	require.NoError(t, err)
	return rdsProxy, rdsProxyDatabase
}

func makeRDSProxyCustomEndpoint(t *testing.T, rdsProxy *rds.DBProxy, name, region string) (*rds.DBProxyEndpoint, types.Database) {
	rdsProxyEndpoint := &rds.DBProxyEndpoint{
		Endpoint:            aws.String("localhost"),
		DBProxyEndpointName: aws.String(name),
		DBProxyName:         rdsProxy.DBProxyName,
		DBProxyEndpointArn:  aws.String(fmt.Sprintf("arn:aws:rds:%v:123456:db-proxy-endpoint:prx-endpoint-%v", region, name)),
		TargetRole:          aws.String(rds.DBProxyEndpointTargetRoleReadOnly),
		Status:              aws.String("available"),
	}
	rdsProxyEndpointDatabase, err := services.NewDatabaseFromRDSProxyCustomEndpoint(rdsProxy, rdsProxyEndpoint, 9999, nil)
	require.NoError(t, err)
	return rdsProxyEndpoint, rdsProxyEndpointDatabase
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
		v := v
		tags[k] = &v
	}
	return tags
}
