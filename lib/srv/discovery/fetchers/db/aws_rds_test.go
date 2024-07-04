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
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
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

	auroraCluster1, auroraCluster1MemberInstance, auroraDatabase1 := makeRDSCluster(t, "cluster-1", "us-east-1", envProdLabels)
	auroraCluster2, auroraCluster2MemberInstance, auroraDatabases2 := makeRDSClusterWithExtraEndpoints(t, "cluster-2", "us-east-2", envDevLabels, true)
	auroraCluster3, auroraCluster3MemberInstance, auroraDatabase3 := makeRDSCluster(t, "cluster-3", "us-east-2", envProdLabels)
	auroraClusterUnsupported, _, _ := makeRDSCluster(t, "serverless", "us-east-1", nil, withRDSClusterEngineMode("serverless"))
	auroraClusterUnavailable, _, _ := makeRDSCluster(t, "cluster-4", "us-east-1", nil, withRDSClusterStatus("creating"))
	auroraClusterUnknownStatus, auroraClusterUnknownStatusMemberInstance, auroraDatabaseUnknownStatus := makeRDSCluster(t, "cluster-5", "us-east-1", nil, withRDSClusterStatus("status-does-not-exist"))
	auroraClusterNoWriter, auroraClusterMemberNoWriter, auroraDatabasesNoWriter := makeRDSClusterWithExtraEndpoints(t, "cluster-6", "us-east-1", envDevLabels, false)

	tests := []awsFetcherTest{
		{
			name: "fetch all",
			inputClients: &cloud.TestCloudClients{
				RDSPerRegion: map[string]rdsiface.RDSAPI{
					"us-east-1": &mocks.RDSMock{
						DBInstances:      []*rds.DBInstance{rdsInstance1, rdsInstance3, auroraCluster1MemberInstance},
						DBClusters:       []*rds.DBCluster{auroraCluster1},
						DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine, postgresEngine},
					},
					"us-east-2": &mocks.RDSMock{
						DBInstances:      []*rds.DBInstance{rdsInstance2, auroraCluster2MemberInstance, auroraCluster3MemberInstance},
						DBClusters:       []*rds.DBCluster{auroraCluster2, auroraCluster3},
						DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine, postgresEngine},
					},
				},
			},
			inputMatchers: []types.AWSMatcher{
				{
					Types:   []string{types.AWSMatcherRDS},
					Regions: []string{"us-east-1"},
					Tags:    toTypeLabels(wildcardLabels),
				},
				{
					Types:   []string{types.AWSMatcherRDS},
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
						DBInstances:      []*rds.DBInstance{rdsInstance1, rdsInstance3, auroraCluster1MemberInstance},
						DBClusters:       []*rds.DBCluster{auroraCluster1},
						DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine, postgresEngine},
					},
					"us-east-2": &mocks.RDSMock{
						DBInstances:      []*rds.DBInstance{rdsInstance2, auroraCluster2MemberInstance, auroraCluster3MemberInstance},
						DBClusters:       []*rds.DBCluster{auroraCluster2, auroraCluster3},
						DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine, postgresEngine},
					},
				},
			},
			inputMatchers: []types.AWSMatcher{
				{
					Types:   []string{types.AWSMatcherRDS},
					Regions: []string{"us-east-1"},
					Tags:    toTypeLabels(envProdLabels),
				},
				{
					Types:   []string{types.AWSMatcherRDS},
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
						DBInstances:      []*rds.DBInstance{rdsInstance1, rdsInstance3, auroraCluster1MemberInstance},
						DBClusters:       []*rds.DBCluster{auroraCluster1},
						DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine},
					},
					"us-east-2": &mocks.RDSMock{
						DBInstances:      []*rds.DBInstance{rdsInstance2, auroraCluster2MemberInstance, auroraCluster3MemberInstance},
						DBClusters:       []*rds.DBCluster{auroraCluster2, auroraCluster3},
						DBEngineVersions: []*rds.DBEngineVersion{postgresEngine},
					},
				},
			},
			inputMatchers: []types.AWSMatcher{
				{
					Types:   []string{types.AWSMatcherRDS},
					Regions: []string{"us-east-1"},
					Tags:    toTypeLabels(wildcardLabels),
				},
				{
					Types:   []string{types.AWSMatcherRDS},
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
						DBInstances:      []*rds.DBInstance{auroraCluster1MemberInstance},
						DBClusters:       []*rds.DBCluster{auroraCluster1, auroraClusterUnsupported},
						DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine},
					},
				},
			},
			inputMatchers: []types.AWSMatcher{{
				Types:   []string{types.AWSMatcherRDS},
				Regions: []string{"us-east-1"},
				Tags:    toTypeLabels(wildcardLabels),
			}},
			wantDatabases: types.Databases{auroraDatabase1},
		},
		{
			name: "skip unavailable databases",
			inputClients: &cloud.TestCloudClients{
				RDS: &mocks.RDSMock{
					DBInstances:      []*rds.DBInstance{rdsInstance1, rdsInstanceUnavailable, rdsInstanceUnknownStatus, auroraCluster1MemberInstance, auroraClusterUnknownStatusMemberInstance},
					DBClusters:       []*rds.DBCluster{auroraCluster1, auroraClusterUnavailable, auroraClusterUnknownStatus},
					DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine, postgresEngine},
				},
			},
			inputMatchers: []types.AWSMatcher{{
				Types:   []string{types.AWSMatcherRDS},
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
					DBInstances:      []*rds.DBInstance{auroraClusterMemberNoWriter},
					DBEngineVersions: []*rds.DBEngineVersion{auroraMySQLEngine},
				},
			},
			inputMatchers: []types.AWSMatcher{{
				Types:   []string{types.AWSMatcherRDS},
				Regions: []string{"us-east-1"},
				Tags:    toTypeLabels(wildcardLabels),
			}},
			wantDatabases: auroraDatabasesNoWriter,
		},
	}
	testAWSFetchers(t, tests...)
}

func makeRDSInstance(t *testing.T, name, region string, labels map[string]string, opts ...func(*rds.DBInstance)) (*rds.DBInstance, types.Database) {
	instance := mocks.RDSInstance(name, region, labels, opts...)
	database, err := common.NewDatabaseFromRDSInstance(instance)
	require.NoError(t, err)
	common.ApplyAWSDatabaseNameSuffix(database, types.AWSMatcherRDS)
	return instance, database
}

func makeRDSCluster(t *testing.T, name, region string, labels map[string]string, opts ...func(*rds.DBCluster)) (*rds.DBCluster, *rds.DBInstance, types.Database) {
	cluster := mocks.RDSCluster(name, region, labels, opts...)
	dbInstanceMember := makeRDSMemberForCluster(t, name, region, "vpc-123", *cluster.Engine, labels)
	database, err := common.NewDatabaseFromRDSCluster(cluster, []*rds.DBInstance{dbInstanceMember})
	require.NoError(t, err)
	common.ApplyAWSDatabaseNameSuffix(database, types.AWSMatcherRDS)
	return cluster, dbInstanceMember, database
}

func makeRDSMemberForCluster(t *testing.T, name, region, vpcid, engine string, labels map[string]string) *rds.DBInstance {
	instanceRDSMember, _ := makeRDSInstance(t, name+"-instance-1", region, labels, func(d *rds.DBInstance) {
		if d.DBSubnetGroup == nil {
			d.DBSubnetGroup = &rds.DBSubnetGroup{}
		}
		d.DBSubnetGroup.SetVpcId(vpcid)
		d.DBClusterIdentifier = aws.String(name)
		d.Engine = aws.String(engine)
	})

	return instanceRDSMember
}

func makeRDSClusterWithExtraEndpoints(t *testing.T, name, region string, labels map[string]string, hasWriter bool) (*rds.DBCluster, *rds.DBInstance, types.Databases) {
	cluster := mocks.RDSCluster(name, region, labels,
		func(cluster *rds.DBCluster) {
			// Disable writer by default. If hasWriter, writer endpoint will be added below.
			cluster.DBClusterMembers = nil
		},
		mocks.WithRDSClusterReader,
		mocks.WithRDSClusterCustomEndpoint("custom1"),
		mocks.WithRDSClusterCustomEndpoint("custom2"),
	)

	var databases types.Databases

	instanceRDSMember := makeRDSMemberForCluster(t, name, region, "vpc-123", aws.StringValue(cluster.Engine), labels)
	dbInstanceMembers := []*rds.DBInstance{instanceRDSMember}

	if hasWriter {
		cluster.DBClusterMembers = append(cluster.DBClusterMembers, &rds.DBClusterMember{
			IsClusterWriter: aws.Bool(true), // Add writer.
		})

		primaryDatabase, err := common.NewDatabaseFromRDSCluster(cluster, dbInstanceMembers)
		require.NoError(t, err)
		databases = append(databases, primaryDatabase)
	}

	readerDatabase, err := common.NewDatabaseFromRDSClusterReaderEndpoint(cluster, dbInstanceMembers)
	require.NoError(t, err)
	databases = append(databases, readerDatabase)

	customDatabases, err := common.NewDatabasesFromRDSClusterCustomEndpoints(cluster, dbInstanceMembers)
	require.NoError(t, err)
	databases = append(databases, customDatabases...)

	for _, db := range databases {
		common.ApplyAWSDatabaseNameSuffix(db, types.AWSMatcherRDS)
	}
	return cluster, instanceRDSMember, databases
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
