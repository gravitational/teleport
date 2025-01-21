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

package usertasks_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types/usertasks"
)

func TestValidateUserTask(t *testing.T) {
	t.Parallel()

	exampleInstanceID := "i-123"

	baseEC2DiscoverTask := func(t *testing.T) *usertasksv1.UserTask {
		userTask, err := usertasks.NewDiscoverEC2UserTask(&usertasksv1.UserTaskSpec{
			Integration: "my-integration",
			TaskType:    "discover-ec2",
			IssueType:   "ec2-ssm-invocation-failure",
			State:       "OPEN",
			DiscoverEc2: &usertasksv1.DiscoverEC2{
				AccountId: "123456789012",
				Region:    "us-east-1",
				Instances: map[string]*usertasksv1.DiscoverEC2Instance{
					exampleInstanceID: {
						InstanceId:      exampleInstanceID,
						DiscoveryConfig: "dc01",
						DiscoveryGroup:  "dg01",
						SyncTime:        timestamppb.Now(),
					},
				},
			},
		})
		require.NoError(t, err)
		return userTask
	}

	exampleClusterName := "MyCluster"
	baseEKSDiscoverTask := func(t *testing.T) *usertasksv1.UserTask {
		userTask, err := usertasks.NewDiscoverEKSUserTask(&usertasksv1.UserTaskSpec{
			Integration: "my-integration",
			TaskType:    "discover-eks",
			IssueType:   "eks-agent-not-connecting",
			State:       "OPEN",
			DiscoverEks: &usertasksv1.DiscoverEKS{
				AccountId: "123456789012",
				Region:    "us-east-1",
				Clusters: map[string]*usertasksv1.DiscoverEKSCluster{
					exampleClusterName: {
						Name:            exampleClusterName,
						DiscoveryConfig: "dc01",
						DiscoveryGroup:  "dg01",
						SyncTime:        timestamppb.Now(),
					},
				},
			},
		})
		require.NoError(t, err)
		return userTask
	}

	exampleDatabaseName := "my-db"
	baseRDSDiscoverTask := func(t *testing.T) *usertasksv1.UserTask {
		userTask, err := usertasks.NewDiscoverRDSUserTask(&usertasksv1.UserTaskSpec{
			Integration: "my-integration",
			TaskType:    "discover-rds",
			IssueType:   "rds-iam-auth-disabled",
			State:       "OPEN",
			DiscoverRds: &usertasksv1.DiscoverRDS{
				AccountId: "123456789012",
				Region:    "us-east-1",
				Databases: map[string]*usertasksv1.DiscoverRDSDatabase{
					exampleDatabaseName: {
						Name:            exampleDatabaseName,
						DiscoveryConfig: "dc01",
						DiscoveryGroup:  "dg01",
						IsCluster:       true,
						Engine:          "aurora-postgresql",
						SyncTime:        timestamppb.Now(),
					},
				},
			},
		})
		require.NoError(t, err)
		return userTask
	}

	tests := []struct {
		name    string
		task    func(t *testing.T) *usertasksv1.UserTask
		wantErr require.ErrorAssertionFunc
	}{
		{
			name: "nil user task",
			task: func(t *testing.T) *usertasksv1.UserTask {
				return nil
			},
			wantErr: require.Error,
		},
		{
			name: "invalid task type",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				ut.Spec.TaskType = "invalid"
				return ut
			},
			wantErr: require.Error,
		},
		{
			name:    "DiscoverEC2: valid",
			task:    baseEC2DiscoverTask,
			wantErr: require.NoError,
		},
		{
			name: "DiscoverEC2: invalid state",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				ut.Spec.State = "invalid"
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEC2: invalid issue type",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				ut.Spec.IssueType = "unknown error"
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEC2: missing integration",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				ut.Spec.Integration = ""
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEC2: missing discover ec2 field",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				ut.Spec.DiscoverEc2 = nil
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEC2: wrong task name",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				ut.Metadata.Name = "another-name"
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEC2: missing account id",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				ut.Spec.DiscoverEc2.AccountId = ""
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEC2: missing region",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				ut.Spec.DiscoverEc2.Region = ""
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEC2: instances - missing instance id in map key",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				origInstanceMetadata := ut.Spec.DiscoverEc2.Instances[exampleInstanceID]
				ut.Spec.DiscoverEc2.Instances[""] = origInstanceMetadata
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEC2: instances - missing instance id in instance metadata",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				origInstanceMetadata := ut.Spec.DiscoverEc2.Instances[exampleInstanceID]
				origInstanceMetadata.InstanceId = ""
				ut.Spec.DiscoverEc2.Instances[exampleInstanceID] = origInstanceMetadata
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEC2: instances - different instance id",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				origInstanceMetadata := ut.Spec.DiscoverEc2.Instances[exampleInstanceID]
				origInstanceMetadata.InstanceId = "i-000"
				ut.Spec.DiscoverEc2.Instances[exampleInstanceID] = origInstanceMetadata
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEC2: instances - missing discovery config",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				origInstanceMetadata := ut.Spec.DiscoverEc2.Instances[exampleInstanceID]
				origInstanceMetadata.DiscoveryConfig = ""
				ut.Spec.DiscoverEc2.Instances[exampleInstanceID] = origInstanceMetadata
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEC2: instances - missing discovery group",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				origInstanceMetadata := ut.Spec.DiscoverEc2.Instances[exampleInstanceID]
				origInstanceMetadata.DiscoveryGroup = ""
				ut.Spec.DiscoverEc2.Instances[exampleInstanceID] = origInstanceMetadata
				return ut
			},
			wantErr: require.Error,
		},
		{
			name:    "DiscoverEKS: valid",
			task:    baseEKSDiscoverTask,
			wantErr: require.NoError,
		},
		{
			name: "DiscoverEKS: invalid issue type",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEKSDiscoverTask(t)
				ut.Spec.IssueType = "unknown error"
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEKS: missing integration",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEKSDiscoverTask(t)
				ut.Spec.Integration = ""
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEKS: missing discover eks field",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEKSDiscoverTask(t)
				ut.Spec.DiscoverEks = nil
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEKS: wrong task name",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEKSDiscoverTask(t)
				ut.Metadata.Name = "another-name"
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEKS: missing account id",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEKSDiscoverTask(t)
				ut.Spec.DiscoverEks.AccountId = ""
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEKS: missing region",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEKSDiscoverTask(t)
				ut.Spec.DiscoverEks.Region = ""
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEKS: clusters - missing cluster name in map key",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEKSDiscoverTask(t)
				origClusterMetadata := ut.Spec.DiscoverEks.Clusters[exampleClusterName]
				ut.Spec.DiscoverEks.Clusters[""] = origClusterMetadata
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEKS: clusters - missing cluster name in cluster metadata",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEKSDiscoverTask(t)
				origClusterMetadata := ut.Spec.DiscoverEks.Clusters[exampleClusterName]
				origClusterMetadata.Name = ""
				ut.Spec.DiscoverEks.Clusters[exampleClusterName] = origClusterMetadata
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEKS: clusters - different cluster name",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEKSDiscoverTask(t)
				origClusterMetadata := ut.Spec.DiscoverEks.Clusters[exampleClusterName]
				origClusterMetadata.Name = "another-cluster"
				ut.Spec.DiscoverEks.Clusters[exampleClusterName] = origClusterMetadata
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEKS: clusters - missing discovery config",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEKSDiscoverTask(t)
				origClusterMetadata := ut.Spec.DiscoverEks.Clusters[exampleClusterName]
				origClusterMetadata.DiscoveryConfig = ""
				ut.Spec.DiscoverEks.Clusters[exampleClusterName] = origClusterMetadata
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverEKS: clusters - missing discovery group",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEKSDiscoverTask(t)
				origClusterMetadata := ut.Spec.DiscoverEks.Clusters[exampleClusterName]
				origClusterMetadata.DiscoveryGroup = ""
				ut.Spec.DiscoverEks.Clusters[exampleClusterName] = origClusterMetadata
				return ut
			},
			wantErr: require.Error,
		},
		{
			name:    "DiscoverRDS: valid",
			task:    baseRDSDiscoverTask,
			wantErr: require.NoError,
		},
		{
			name: "DiscoverRDS: invalid issue type",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseRDSDiscoverTask(t)
				ut.Spec.IssueType = "unknown error"
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverRDS: missing integration",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseRDSDiscoverTask(t)
				ut.Spec.Integration = ""
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverRDS: missing discover rds field",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseRDSDiscoverTask(t)
				ut.Spec.DiscoverRds = nil
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverRDS: wrong task name",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseRDSDiscoverTask(t)
				ut.Metadata.Name = "another-name"
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverRDS: missing account id",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseRDSDiscoverTask(t)
				ut.Spec.DiscoverRds.AccountId = ""
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverRDS: missing region",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseRDSDiscoverTask(t)
				ut.Spec.DiscoverRds.Region = ""
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverRDS: databases - missing database name in map key",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseRDSDiscoverTask(t)
				origDatabasdeMetadata := ut.Spec.DiscoverRds.Databases[exampleDatabaseName]
				ut.Spec.DiscoverRds.Databases[""] = origDatabasdeMetadata
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverRDS: databases - missing database name in metadata",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseRDSDiscoverTask(t)
				origDatabasdeMetadata := ut.Spec.DiscoverRds.Databases[exampleDatabaseName]
				origDatabasdeMetadata.Name = ""
				ut.Spec.DiscoverRds.Databases[exampleDatabaseName] = origDatabasdeMetadata
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverRDS: databases - different database name",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseRDSDiscoverTask(t)
				origDatabasdeMetadata := ut.Spec.DiscoverRds.Databases[exampleDatabaseName]
				origDatabasdeMetadata.Name = "another-database"
				ut.Spec.DiscoverRds.Databases[exampleDatabaseName] = origDatabasdeMetadata
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverRDS: databases - missing discovery config",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseRDSDiscoverTask(t)
				origDatabasdeMetadata := ut.Spec.DiscoverRds.Databases[exampleDatabaseName]
				origDatabasdeMetadata.DiscoveryConfig = ""
				ut.Spec.DiscoverRds.Databases[exampleDatabaseName] = origDatabasdeMetadata
				return ut
			},
			wantErr: require.Error,
		},
		{
			name: "DiscoverRDS: databases - missing discovery group",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseRDSDiscoverTask(t)
				origDatabasdeMetadata := ut.Spec.DiscoverRds.Databases[exampleDatabaseName]
				origDatabasdeMetadata.DiscoveryGroup = ""
				ut.Spec.DiscoverRds.Databases[exampleDatabaseName] = origDatabasdeMetadata
				return ut
			},
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := usertasks.ValidateUserTask(tt.task(t))
			tt.wantErr(t, err)
		})
	}
}

func TestNewDiscoverEC2UserTask(t *testing.T) {
	t.Parallel()

	userTaskExpirationTime := time.Now()
	userTaskExpirationTimestamp := timestamppb.New(userTaskExpirationTime)
	instanceSyncTimestamp := userTaskExpirationTimestamp

	baseEC2DiscoverTaskSpec := &usertasksv1.UserTaskSpec{
		Integration: "my-integration",
		TaskType:    "discover-ec2",
		IssueType:   "ec2-ssm-invocation-failure",
		State:       "OPEN",
		DiscoverEc2: &usertasksv1.DiscoverEC2{
			AccountId: "123456789012",
			Region:    "us-east-1",
			Instances: map[string]*usertasksv1.DiscoverEC2Instance{
				"i-123": {
					InstanceId:      "i-123",
					DiscoveryConfig: "dc01",
					DiscoveryGroup:  "dg01",
					SyncTime:        instanceSyncTimestamp,
				},
			},
		},
	}

	tests := []struct {
		name         string
		taskSpec     *usertasksv1.UserTaskSpec
		taskOption   []usertasks.UserTaskOption
		expectedTask *usertasksv1.UserTask
	}{
		{
			name:     "options are applied task type",
			taskSpec: baseEC2DiscoverTaskSpec,
			expectedTask: &usertasksv1.UserTask{
				Kind:    "user_task",
				Version: "v1",
				Metadata: &headerv1.Metadata{
					Name:    "f36b8798-fdec-59fe-8bd0-33f4890ced05",
					Expires: userTaskExpirationTimestamp,
				},
				Spec: baseEC2DiscoverTaskSpec,
			},
			taskOption: []usertasks.UserTaskOption{
				usertasks.WithExpiration(userTaskExpirationTime),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTask, err := usertasks.NewDiscoverEC2UserTask(tt.taskSpec, tt.taskOption...)
			require.NoError(t, err)
			require.Equal(t, tt.expectedTask, gotTask)
		})
	}
}

func TestNewDiscoverEKSUserTask(t *testing.T) {
	t.Parallel()

	userTaskExpirationTime := time.Now()
	userTaskExpirationTimestamp := timestamppb.New(userTaskExpirationTime)
	clusterSyncTimestamp := userTaskExpirationTimestamp

	baseEKSDiscoverTaskSpec := &usertasksv1.UserTaskSpec{
		Integration: "my-integration",
		TaskType:    "discover-eks",
		IssueType:   "eks-agent-not-connecting",
		State:       "OPEN",
		DiscoverEks: &usertasksv1.DiscoverEKS{
			AccountId: "123456789012",
			Region:    "us-east-1",
			Clusters: map[string]*usertasksv1.DiscoverEKSCluster{
				"MyKubeCluster": {
					Name:            "MyKubeCluster",
					DiscoveryConfig: "dc01",
					DiscoveryGroup:  "dg01",
					SyncTime:        clusterSyncTimestamp,
				},
			},
		},
	}

	tests := []struct {
		name         string
		taskSpec     *usertasksv1.UserTaskSpec
		taskOption   []usertasks.UserTaskOption
		expectedTask *usertasksv1.UserTask
	}{
		{
			name:     "options are applied task type",
			taskSpec: baseEKSDiscoverTaskSpec,
			expectedTask: &usertasksv1.UserTask{
				Kind:    "user_task",
				Version: "v1",
				Metadata: &headerv1.Metadata{
					Name:    "09b7d37e-3570-531a-b326-1860cafc23fb",
					Expires: userTaskExpirationTimestamp,
				},
				Spec: baseEKSDiscoverTaskSpec,
			},
			taskOption: []usertasks.UserTaskOption{
				usertasks.WithExpiration(userTaskExpirationTime),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTask, err := usertasks.NewDiscoverEKSUserTask(tt.taskSpec, tt.taskOption...)
			require.NoError(t, err)
			require.Equal(t, tt.expectedTask, gotTask)
		})
	}
}

func TestNewDiscoverRDSUserTask(t *testing.T) {
	t.Parallel()

	userTaskExpirationTime := time.Now()
	userTaskExpirationTimestamp := timestamppb.New(userTaskExpirationTime)
	databaseSyncTimestamp := userTaskExpirationTimestamp

	baseRDSDiscoverTaskSpec := &usertasksv1.UserTaskSpec{
		Integration: "my-integration",
		TaskType:    "discover-rds",
		IssueType:   "rds-iam-auth-disabled",
		State:       "OPEN",
		DiscoverRds: &usertasksv1.DiscoverRDS{
			AccountId: "123456789012",
			Region:    "us-east-1",
			Databases: map[string]*usertasksv1.DiscoverRDSDatabase{
				"my-database": {
					Name:            "my-database",
					DiscoveryConfig: "dc01",
					DiscoveryGroup:  "dg01",
					SyncTime:        databaseSyncTimestamp,
					IsCluster:       true,
					Engine:          "aurora-postgresql",
				},
			},
		},
	}

	tests := []struct {
		name         string
		taskSpec     *usertasksv1.UserTaskSpec
		taskOption   []usertasks.UserTaskOption
		expectedTask *usertasksv1.UserTask
	}{
		{
			name:     "options are applied task type",
			taskSpec: baseRDSDiscoverTaskSpec,
			expectedTask: &usertasksv1.UserTask{
				Kind:    "user_task",
				Version: "v1",
				Metadata: &headerv1.Metadata{
					Name:    "8c6014e2-8275-54d7-b285-31e0194b7835",
					Expires: userTaskExpirationTimestamp,
				},
				Spec: baseRDSDiscoverTaskSpec,
			},
			taskOption: []usertasks.UserTaskOption{
				usertasks.WithExpiration(userTaskExpirationTime),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTask, err := usertasks.NewDiscoverRDSUserTask(tt.taskSpec, tt.taskOption...)
			require.NoError(t, err)
			require.Equal(t, tt.expectedTask, gotTask)
		})
	}
}
