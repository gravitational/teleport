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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/cloud"
	"github.com/gravitational/teleport/lib/srv/db/common"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/aws/aws-sdk-go/service/redshift"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestWatcher tests cloud databases watcher.
func TestWatcher(t *testing.T) {
	ctx := context.Background()

	rdsInstance1, rdsDatabase1 := makeRDSInstance(t, "instance-1", "us-east-1", map[string]string{"env": "prod"})
	rdsInstance2, _ := makeRDSInstance(t, "instance-2", "us-east-2", map[string]string{"env": "prod"})
	rdsInstance3, _ := makeRDSInstance(t, "instance-3", "us-east-1", map[string]string{"env": "dev"})
	rdsInstance4, rdsDatabase4 := makeRDSInstance(t, "instance-4", "us-west-1", nil)

	auroraCluster1, auroraDatabase1 := makeRDSCluster(t, "cluster-1", "us-east-1", services.RDSEngineModeProvisioned, map[string]string{"env": "prod"})
	auroraCluster2, auroraDatabases2 := makeRDSClusterWithExtraEndpoints(t, "cluster-2", "us-east-2", map[string]string{"env": "dev"})
	auroraCluster3, _ := makeRDSCluster(t, "cluster-3", "us-east-2", services.RDSEngineModeProvisioned, map[string]string{"env": "prod"})
	auroraClusterUnsupported, _ := makeRDSCluster(t, "serverless", "us-east-1", services.RDSEngineModeServerless, map[string]string{"env": "prod"})

	redshiftUse1Prod, redshiftDatabaseUse1Prod := makeRedshiftCluster(t, "us-east-1", "prod")
	redshiftUse1Dev, _ := makeRedshiftCluster(t, "us-east-1", "dev")

	tests := []struct {
		name              string
		awsMatchers       []services.AWSMatcher
		clients           common.CloudClients
		expectedDatabases types.Databases
	}{
		{
			name: "rds labels matching",
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
			name: "rds aurora unsupported",
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
			name: "redshift",
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
			name: "matcher with multiple types",
			awsMatchers: []services.AWSMatcher{
				{
					Types:   []string{services.AWSMatcherRedshift, services.AWSMatcherRDS},
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
			},
			expectedDatabases: types.Databases{auroraDatabase1, redshiftDatabaseUse1Prod},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			watcher, err := NewWatcher(ctx, WatcherConfig{AWSMatchers: test.awsMatchers, Clients: test.clients})
			require.NoError(t, err)

			go watcher.fetchAndSend()
			select {
			case databases := <-watcher.DatabasesC():
				require.Equal(t, test.expectedDatabases, databases)
			case <-time.After(time.Second):
				t.Fatal("didn't receive databases after 1 second")
			}
		})
	}
}

func makeRDSInstance(t *testing.T, name, region string, labels map[string]string) (*rds.DBInstance, types.Database) {
	instance := &rds.DBInstance{
		DBInstanceArn:        aws.String(fmt.Sprintf("arn:aws:rds:%v:1234567890:db:%v", region, name)),
		DBInstanceIdentifier: aws.String(name),
		DbiResourceId:        aws.String(uuid.New().String()),
		Engine:               aws.String(services.RDSEnginePostgres),
		Endpoint: &rds.Endpoint{
			Address: aws.String("localhost"),
			Port:    aws.Int64(5432),
		},
		TagList: labelsToTags(labels),
	}
	database, err := services.NewDatabaseFromRDSInstance(instance)
	require.NoError(t, err)
	return instance, database
}

func makeRDSCluster(t *testing.T, name, region, engineMode string, labels map[string]string) (*rds.DBCluster, types.Database) {
	cluster := &rds.DBCluster{
		DBClusterArn:        aws.String(fmt.Sprintf("arn:aws:rds:%v:1234567890:cluster:%v", region, name)),
		DBClusterIdentifier: aws.String(name),
		DbClusterResourceId: aws.String(uuid.New().String()),
		Engine:              aws.String(services.RDSEngineAuroraMySQL),
		EngineMode:          aws.String(engineMode),
		Endpoint:            aws.String("localhost"),
		Port:                aws.Int64(3306),
		TagList:             labelsToTags(labels),
	}
	database, err := services.NewDatabaseFromRDSCluster(cluster)
	require.NoError(t, err)
	return cluster, database
}

func makeRedshiftCluster(t *testing.T, region, env string) (*redshift.Cluster, types.Database) {
	cluster := &redshift.Cluster{
		ClusterIdentifier:   aws.String(env),
		ClusterNamespaceArn: aws.String(fmt.Sprintf("arn:aws:redshift:%s:1234567890:namespace:%s", region, env)),
		Endpoint: &redshift.Endpoint{
			Address: aws.String("localhost"),
			Port:    aws.Int64(5439),
		},
		Tags: []*redshift.Tag{{
			Key:   aws.String("env"),
			Value: aws.String(env),
		}},
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

func labelsToTags(labels map[string]string) (tags []*rds.Tag) {
	for key, val := range labels {
		tags = append(tags, &rds.Tag{
			Key:   aws.String(key),
			Value: aws.String(val),
		})
	}
	return tags
}
