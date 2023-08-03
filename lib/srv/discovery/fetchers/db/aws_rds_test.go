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
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// TestRDSFetchers tests RDS instance fetcher and Aurora cluster fetcher (as
// they share the same matcher type).
func TestRDSFetchers(t *testing.T) {
	t.Parallel()

	auroraMySQLEngine := &rds.DBEngineVersion{Engine: aws.String(services.RDSEngineAuroraMySQL)}
	postgresEngine := &rds.DBEngineVersion{Engine: aws.String(services.RDSEnginePostgres)}

	rdsInstance1, rdsDatabase1 := makeRDSInstance(t, "instance-1", "us-east-1", envProdLabels)
	rdsInstance2, rdsDatabase2 := makeRDSInstance(t, "instance-2", "us-east-2", envProdLabels)
	rdsInstance3, rdsDatabase3 := makeRDSInstance(t, "instance-3", "us-east-1", envDevLabels)
	rdsInstanceUnavailable, _ := makeRDSInstance(t, "instance-5", "us-west-1", nil, withRDSInstanceStatus("stopped"))
	rdsInstanceUnknownStatus, rdsDatabaseUnknownStatus := makeRDSInstance(t, "instance-5", "us-west-6", nil, withRDSInstanceStatus("status-does-not-exist"))

	auroraCluster1, auroraDatabase1 := makeRDSCluster(t, "cluster-1", "us-east-1", envProdLabels)
	auroraCluster2, auroraDatabases2 := makeRDSClusterWithExtraEndpoints(t, "cluster-2", "us-east-2", envDevLabels, true)
	auroraCluster3, auroraDatabase3 := makeRDSCluster(t, "cluster-3", "us-east-2", envProdLabels)
	auroraClusterUnsupported, _ := makeRDSCluster(t, "serverless", "us-east-1", nil, withRDSClusterEngineMode("serverless"))
	auroraClusterUnavailable, _ := makeRDSCluster(t, "cluster-4", "us-east-1", nil, withRDSClusterStatus("creating"))
	auroraClusterUnknownStatus, auroraDatabaseUnknownStatus := makeRDSCluster(t, "cluster-5", "us-east-1", nil, withRDSClusterStatus("status-does-not-exist"))
	auroraClusterNoWriter, auroraDatabasesNoWriter := makeRDSClusterWithExtraEndpoints(t, "cluster-6", "us-east-1", envDevLabels, false)

	tests := []awsFetcherTest{
		{
			name: "fetch all",
			inputClients: &cloud.TestCloudClients{
				RDSPerRegion: map[string]rdsiface.RDSAPI{
					"us-east-1": &mocks.RDSMock{
						DBInstances:      []*rds.DBInstance{rdsInstance1, rdsInstance3},
						DBClusters:       []*rds.DBCluster{auroraCluster1},
						DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine, postgresEngine},
					},
					"us-east-2": &mocks.RDSMock{
						DBInstances:      []*rds.DBInstance{rdsInstance2},
						DBClusters:       []*rds.DBCluster{auroraCluster2, auroraCluster3},
						DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine, postgresEngine},
					},
				},
			},
			inputMatchers: []types.AWSMatcher{
				{
					Types:   []string{services.AWSMatcherRDS},
					Regions: []string{"us-east-1"},
					Tags:    toTypeLabels(wildcardLabels),
				},
				{
					Types:   []string{services.AWSMatcherRDS},
					Regions: []string{"us-east-2"},
					Tags:    toTypeLabels(wildcardLabels),
				},
			},
			wantDatabases: append(types.Databases{
				rdsDatabase1, rdsDatabase2, rdsDatabase3,
				auroraDatabase1, auroraDatabase3,
			}, auroraDatabases2...),
		},
		{
			name: "fetch different labels for different regions",
			inputClients: &cloud.TestCloudClients{
				RDSPerRegion: map[string]rdsiface.RDSAPI{
					"us-east-1": &mocks.RDSMock{
						DBInstances:      []*rds.DBInstance{rdsInstance1, rdsInstance3},
						DBClusters:       []*rds.DBCluster{auroraCluster1},
						DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine, postgresEngine},
					},
					"us-east-2": &mocks.RDSMock{
						DBInstances:      []*rds.DBInstance{rdsInstance2},
						DBClusters:       []*rds.DBCluster{auroraCluster2, auroraCluster3},
						DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine, postgresEngine},
					},
				},
			},
			inputMatchers: []types.AWSMatcher{
				{
					Types:   []string{services.AWSMatcherRDS},
					Regions: []string{"us-east-1"},
					Tags:    toTypeLabels(envProdLabels),
				},
				{
					Types:   []string{services.AWSMatcherRDS},
					Regions: []string{"us-east-2"},
					Tags:    toTypeLabels(envDevLabels),
				},
			},
			wantDatabases: append(types.Databases{
				rdsDatabase1,
				auroraDatabase1,
			}, auroraDatabases2...),
		},
		{
			name: "skip unrecognized engines",
			inputClients: &cloud.TestCloudClients{
				RDSPerRegion: map[string]rdsiface.RDSAPI{
					"us-east-1": &mocks.RDSMock{
						DBInstances:      []*rds.DBInstance{rdsInstance1, rdsInstance3},
						DBClusters:       []*rds.DBCluster{auroraCluster1},
						DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine},
					},
					"us-east-2": &mocks.RDSMock{
						DBInstances:      []*rds.DBInstance{rdsInstance2},
						DBClusters:       []*rds.DBCluster{auroraCluster2, auroraCluster3},
						DBEngineVersions: []*rds.DBEngineVersion{postgresEngine},
					},
				},
			},
			inputMatchers: []types.AWSMatcher{
				{
					Types:   []string{services.AWSMatcherRDS},
					Regions: []string{"us-east-1"},
					Tags:    toTypeLabels(wildcardLabels),
				},
				{
					Types:   []string{services.AWSMatcherRDS},
					Regions: []string{"us-east-2"},
					Tags:    toTypeLabels(wildcardLabels),
				},
			},
			wantDatabases: types.Databases{rdsDatabase2, auroraDatabase1},
		},
		{
			name: "skip unsupported databases",
			inputClients: &cloud.TestCloudClients{
				RDSPerRegion: map[string]rdsiface.RDSAPI{
					"us-east-1": &mocks.RDSMock{
						DBClusters:       []*rds.DBCluster{auroraCluster1, auroraClusterUnsupported},
						DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine},
					},
				},
			},
			inputMatchers: []types.AWSMatcher{{
				Types:   []string{services.AWSMatcherRDS},
				Regions: []string{"us-east-1"},
				Tags:    toTypeLabels(wildcardLabels),
			}},
			wantDatabases: types.Databases{auroraDatabase1},
		},
		{
			name: "skip unavailable databases",
			inputClients: &cloud.TestCloudClients{
				RDS: &mocks.RDSMock{
					DBInstances:      []*rds.DBInstance{rdsInstance1, rdsInstanceUnavailable, rdsInstanceUnknownStatus},
					DBClusters:       []*rds.DBCluster{auroraCluster1, auroraClusterUnavailable, auroraClusterUnknownStatus},
					DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine, postgresEngine},
				},
			},
			inputMatchers: []types.AWSMatcher{{
				Types:   []string{services.AWSMatcherRDS},
				Regions: []string{"us-east-1"},
				Tags:    toTypeLabels(wildcardLabels),
			}},
			wantDatabases: types.Databases{rdsDatabase1, rdsDatabaseUnknownStatus, auroraDatabase1, auroraDatabaseUnknownStatus},
		},
		{
			name: "Aurora cluster without writer",
			inputClients: &cloud.TestCloudClients{
				RDS: &mocks.RDSMock{
					DBClusters:       []*rds.DBCluster{auroraClusterNoWriter},
					DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine},
				},
			},
			inputMatchers: []types.AWSMatcher{{
				Types:   []string{services.AWSMatcherRDS},
				Regions: []string{"us-east-1"},
				Tags:    toTypeLabels(wildcardLabels),
			}},
			wantDatabases: auroraDatabasesNoWriter,
		},
	}
	testAWSFetchers(t, tests...)
}

func makeRDSInstance(t *testing.T, name, region string, labels map[string]string, opts ...func(*rds.DBInstance)) (*rds.DBInstance, types.Database) {
	instance := &rds.DBInstance{
		DBInstanceArn:        aws.String(fmt.Sprintf("arn:aws:rds:%v:123456789012:db:%v", region, name)),
		DBInstanceIdentifier: aws.String(name),
		DbiResourceId:        aws.String(uuid.New().String()),
		Engine:               aws.String(services.RDSEnginePostgres),
		DBInstanceStatus:     aws.String("available"),
		Endpoint: &rds.Endpoint{
			Address: aws.String("localhost"),
			Port:    aws.Int64(5432),
		},
		TagList: libcloudaws.LabelsToTags[rds.Tag](labels),
	}
	for _, opt := range opts {
		opt(instance)
	}

	database, err := services.NewDatabaseFromRDSInstance(instance)
	require.NoError(t, err)
	common.ApplyAWSDatabaseNameSuffix(database, services.AWSMatcherRDS)
	return instance, database
}

func makeRDSCluster(t *testing.T, name, region string, labels map[string]string, opts ...func(*rds.DBCluster)) (*rds.DBCluster, types.Database) {
	cluster := &rds.DBCluster{
		DBClusterArn:        aws.String(fmt.Sprintf("arn:aws:rds:%v:123456789012:cluster:%v", region, name)),
		DBClusterIdentifier: aws.String(name),
		DbClusterResourceId: aws.String(uuid.New().String()),
		Engine:              aws.String(services.RDSEngineAuroraMySQL),
		EngineMode:          aws.String(services.RDSEngineModeProvisioned),
		Status:              aws.String("available"),
		Endpoint:            aws.String("localhost"),
		Port:                aws.Int64(3306),
		TagList:             libcloudaws.LabelsToTags[rds.Tag](labels),
		DBClusterMembers: []*rds.DBClusterMember{{
			IsClusterWriter: aws.Bool(true), // Only one writer.
		}},
	}
	for _, opt := range opts {
		opt(cluster)
	}

	database, err := services.NewDatabaseFromRDSCluster(cluster)
	require.NoError(t, err)
	common.ApplyAWSDatabaseNameSuffix(database, services.AWSMatcherRDS)
	return cluster, database
}

func makeRDSClusterWithExtraEndpoints(t *testing.T, name, region string, labels map[string]string, hasWriter bool) (*rds.DBCluster, types.Databases) {
	cluster := &rds.DBCluster{
		DBClusterArn:        aws.String(fmt.Sprintf("arn:aws:rds:%v:123456789012:cluster:%v", region, name)),
		DBClusterIdentifier: aws.String(name),
		DbClusterResourceId: aws.String(uuid.New().String()),
		Engine:              aws.String(services.RDSEngineAuroraMySQL),
		EngineMode:          aws.String(services.RDSEngineModeProvisioned),
		Status:              aws.String("available"),
		Endpoint:            aws.String("localhost"),
		ReaderEndpoint:      aws.String("reader.host"),
		Port:                aws.Int64(3306),
		TagList:             libcloudaws.LabelsToTags[rds.Tag](labels),
		DBClusterMembers: []*rds.DBClusterMember{{
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

	for _, db := range databases {
		common.ApplyAWSDatabaseNameSuffix(db, services.AWSMatcherRDS)
	}
	return cluster, databases
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
