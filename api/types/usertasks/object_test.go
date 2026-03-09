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

package usertasks

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
)

func TestValidateUserTask(t *testing.T) {
	t.Parallel()

	exampleInstanceID := "i-123"

	// baseEC2DiscoverTask uses an SSM issue type which requires account_id and region.
	baseEC2DiscoverTask := func(t *testing.T) *usertasksv1.UserTask {
		userTask, err := NewDiscoverEC2UserTask(&usertasksv1.UserTaskSpec{
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

	// baseEC2PermissionIssueTask uses a permission issue type which does NOT
	// require account_id and region (errors occur before these are known).
	// Permission issues are allowed to have empty instance lists since the
	// error occurs before any instances can be discovered.
	baseEC2PermissionIssueTask := func(t *testing.T) *usertasksv1.UserTask {
		userTask, err := NewDiscoverEC2UserTask(&usertasksv1.UserTaskSpec{
			Integration: "my-integration",
			TaskType:    "discover-ec2",
			IssueType:   "ec2-perm-account-denied",
			State:       "OPEN",
			DiscoverEc2: &usertasksv1.DiscoverEC2{
				AccountId: "123456789012",
				Region:    "us-east-1",
				Instances: map[string]*usertasksv1.DiscoverEC2Instance{},
			},
		})
		require.NoError(t, err)
		return userTask
	}

	exampleClusterName := "MyCluster"
	baseEKSDiscoverTask := func(t *testing.T) *usertasksv1.UserTask {
		userTask, err := NewDiscoverEKSUserTask(&usertasksv1.UserTaskSpec{
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
		userTask, err := NewDiscoverRDSUserTask(&usertasksv1.UserTaskSpec{
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

	exampleVMID := "/subscriptions/sub-123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachines/my-vm"
	baseAzureVMDiscoverTask := func(t *testing.T) *usertasksv1.UserTask {
		userTask, err := NewDiscoverAzureVMUserTask(
			TaskGroup{
				Integration: "my-integration",
				IssueType:   AutoDiscoverAzureVMIssueEnrollmentError,
			},
			time.Now().Add(24*time.Hour),
			&usertasksv1.DiscoverAzureVM{
				SubscriptionId: "sub-123",
				ResourceGroup:  "my-rg",
				Region:         "eastus",
				Instances: map[string]*usertasksv1.DiscoverAzureVMInstance{
					exampleVMID: {
						VmId:            exampleVMID,
						DiscoveryConfig: "dc01",
						DiscoveryGroup:  "dg01",
						SyncTime:        timestamppb.Now(),
					},
				},
			},
		)
		require.NoError(t, err)
		return userTask
	}

	const noError = ""

	tests := []struct {
		name    string
		task    func(t *testing.T) *usertasksv1.UserTask
		wantErr string
	}{
		{
			name: "nil user task",
			task: func(t *testing.T) *usertasksv1.UserTask {
				return nil
			},
			wantErr: "invalid kind",
		},
		{
			name: "invalid task type",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				ut.Spec.TaskType = "invalid"
				return ut
			},
			wantErr: "is not valid",
		},
		{
			name:    "DiscoverEC2: valid",
			task:    baseEC2DiscoverTask,
			wantErr: noError,
		},
		{
			name: "DiscoverEC2: invalid state",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				ut.Spec.State = "invalid"
				return ut
			},
			wantErr: "invalid task state",
		},
		{
			name: "DiscoverEC2: invalid issue type",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				ut.Spec.IssueType = "unknown error"
				ut.Metadata.Name = "1b8320d7-0cc0-53f8-81a5-14a8661a9846"
				return ut
			},
			wantErr: "invalid issue type",
		},
		{
			name: "DiscoverEC2: ec2-perm-account-denied is valid",
			task: func(t *testing.T) *usertasksv1.UserTask {
				return baseEC2PermissionIssueTask(t)
			},
			wantErr: noError,
		},
		{
			name: "DiscoverEC2: ec2-perm-org-denied is valid",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2PermissionIssueTask(t)
				ut.Spec.IssueType = "ec2-perm-org-denied"
				ut.Metadata.Name = TaskNameForDiscoverEC2(TaskNameForDiscoverEC2Parts{
					Integration: ut.Spec.Integration,
					IssueType:   ut.Spec.IssueType,
					AccountID:   ut.Spec.DiscoverEc2.AccountId,
					Region:      ut.Spec.DiscoverEc2.Region,
				})
				return ut
			},
			wantErr: noError,
		},
		{
			name: "DiscoverEC2: missing integration",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				ut.Spec.Integration = ""
				return ut
			},
			wantErr: "integration is required",
		},
		{
			name: "DiscoverEC2: missing discover ec2 field",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				ut.Spec.DiscoverEc2 = nil
				return ut
			},
			wantErr: "discover-ec2 requires the discover_ec2 field",
		},
		{
			name: "DiscoverEC2: wrong task name",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				ut.Metadata.Name = "another-name"
				return ut
			},
			wantErr: "task name is pre-defined for discover-ec2 types",
		},
		{
			name: "DiscoverEC2: missing account id",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				ut.Spec.DiscoverEc2.AccountId = ""
				return ut
			},
			wantErr: "discover-ec2 requires the discover_ec2.account_id field",
		},
		{
			name: "DiscoverEC2: missing region",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				ut.Spec.DiscoverEc2.Region = ""
				return ut
			},
			wantErr: "discover-ec2 requires the discover_ec2.region field",
		},
		{
			name: "DiscoverEC2: ec2-perm-account-denied allows missing region",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2PermissionIssueTask(t)
				ut.Spec.DiscoverEc2.Region = ""
				ut.Metadata.Name = TaskNameForDiscoverEC2(TaskNameForDiscoverEC2Parts{
					Integration: ut.Spec.Integration,
					IssueType:   ut.Spec.IssueType,
					AccountID:   ut.Spec.DiscoverEc2.AccountId,
				})
				return ut
			},
			wantErr: noError,
		},
		{
			name: "DiscoverEC2: ec2-perm-org-denied allows missing region",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2PermissionIssueTask(t)
				ut.Spec.IssueType = "ec2-perm-org-denied"
				ut.Spec.DiscoverEc2.Region = ""
				ut.Metadata.Name = TaskNameForDiscoverEC2(TaskNameForDiscoverEC2Parts{
					Integration: ut.Spec.Integration,
					IssueType:   ut.Spec.IssueType,
					AccountID:   ut.Spec.DiscoverEc2.AccountId,
				})
				return ut
			},
			wantErr: noError,
		},
		{
			name: "DiscoverEC2: ec2-perm-account-denied allows missing account id",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2PermissionIssueTask(t)
				ut.Spec.DiscoverEc2.AccountId = ""
				ut.Metadata.Name = TaskNameForDiscoverEC2(TaskNameForDiscoverEC2Parts{
					Integration: ut.Spec.Integration,
					IssueType:   ut.Spec.IssueType,
					Region:      ut.Spec.DiscoverEc2.Region,
				})
				return ut
			},
			wantErr: noError,
		},
		{
			name: "DiscoverEC2: permission issue allows empty instances",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2PermissionIssueTask(t)
				ut.Spec.DiscoverEc2.Instances = nil
				return ut
			},
			wantErr: noError,
		},
		{
			name: "DiscoverEC2: permission issue validates instances when present",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2PermissionIssueTask(t)
				ut.Spec.DiscoverEc2.Instances = map[string]*usertasksv1.DiscoverEC2Instance{
					exampleInstanceID: {
						InstanceId:      exampleInstanceID,
						DiscoveryConfig: "dc-01",
						DiscoveryGroup:  "dg-01",
					},
				}
				return ut
			},
			wantErr: noError,
		},
		{
			name: "DiscoverEC2: instances - missing instance id in map key",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEC2DiscoverTask(t)
				origInstanceMetadata := ut.Spec.DiscoverEc2.Instances[exampleInstanceID]
				ut.Spec.DiscoverEc2.Instances[""] = origInstanceMetadata
				return ut
			},
			wantErr: "instance id in discover_ec2.instances map is required",
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
			wantErr: "instance id in discover_ec2.instances field is required",
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
			wantErr: "instance id in discover_ec2.instances map and field are different",
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
			wantErr: "discovery config in discover_ec2.instances field is required",
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
			wantErr: "discovery group in discover_ec2.instances field is required",
		},
		{
			name:    "DiscoverEKS: valid",
			task:    baseEKSDiscoverTask,
			wantErr: noError,
		},
		{
			name: "DiscoverEKS: invalid issue type",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEKSDiscoverTask(t)
				ut.Spec.IssueType = "unknown error"
				ut.Metadata.Name = "ebb43107-ea5f-5e6c-a53f-230aa683c4a7"
				return ut
			},
			wantErr: "invalid issue type",
		},
		{
			name: "DiscoverEKS: missing integration",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEKSDiscoverTask(t)
				ut.Spec.Integration = ""
				return ut
			},
			wantErr: "integration is required",
		},
		{
			name: "DiscoverEKS: missing discover eks field",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEKSDiscoverTask(t)
				ut.Spec.DiscoverEks = nil
				return ut
			},
			wantErr: "discover-eks requires the discover_eks field",
		},
		{
			name: "DiscoverEKS: wrong task name",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEKSDiscoverTask(t)
				ut.Metadata.Name = "another-name"
				return ut
			},
			wantErr: "task name is pre-defined for discover-eks types",
		},
		{
			name: "DiscoverEKS: missing account id",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEKSDiscoverTask(t)
				ut.Spec.DiscoverEks.AccountId = ""
				return ut
			},
			wantErr: "discover-eks requires the discover_eks.account_id field",
		},
		{
			name: "DiscoverEKS: missing region",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEKSDiscoverTask(t)
				ut.Spec.DiscoverEks.Region = ""
				return ut
			},
			wantErr: "discover-eks requires the discover_eks.region field",
		},
		{
			name: "DiscoverEKS: clusters - missing cluster name in map key",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseEKSDiscoverTask(t)
				origClusterMetadata := ut.Spec.DiscoverEks.Clusters[exampleClusterName]
				ut.Spec.DiscoverEks.Clusters[""] = origClusterMetadata
				return ut
			},
			wantErr: "cluster name in discover_eks.clusters map is required",
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
			wantErr: "cluster name in discover_eks.clusters field is required",
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
			wantErr: "cluster name in discover_eks.clusters map and field are different",
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
			wantErr: "discovery config in discover_eks.clusters field is required",
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
			wantErr: "discovery group in discover_eks.clusters field is required",
		},
		{
			name:    "DiscoverRDS: valid",
			task:    baseRDSDiscoverTask,
			wantErr: noError,
		},
		{
			name: "DiscoverRDS: invalid issue type",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseRDSDiscoverTask(t)
				ut.Spec.IssueType = "unknown error"
				ut.Metadata.Name = "8f7bd657-fd2a-507d-bc7b-c42593ec78f6"
				return ut
			},
			wantErr: "invalid issue type",
		},
		{
			name: "DiscoverRDS: missing integration",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseRDSDiscoverTask(t)
				ut.Spec.Integration = ""
				return ut
			},
			wantErr: "integration is required",
		},
		{
			name: "DiscoverRDS: missing discover rds field",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseRDSDiscoverTask(t)
				ut.Spec.DiscoverRds = nil
				return ut
			},
			wantErr: "discover-rds requires the discover_rds field",
		},
		{
			name: "DiscoverRDS: wrong task name",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseRDSDiscoverTask(t)
				ut.Metadata.Name = "another-name"
				return ut
			},
			wantErr: "task name is pre-defined for discover-rds types",
		},
		{
			name: "DiscoverRDS: missing account id",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseRDSDiscoverTask(t)
				ut.Spec.DiscoverRds.AccountId = ""
				return ut
			},
			wantErr: "discover-rds requires the discover_rds.account_id field",
		},
		{
			name: "DiscoverRDS: missing region",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseRDSDiscoverTask(t)
				ut.Spec.DiscoverRds.Region = ""
				return ut
			},
			wantErr: "discover-rds requires the discover_rds.region field",
		},
		{
			name: "DiscoverRDS: databases - missing database name in map key",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseRDSDiscoverTask(t)
				origDatabasdeMetadata := ut.Spec.DiscoverRds.Databases[exampleDatabaseName]
				ut.Spec.DiscoverRds.Databases[""] = origDatabasdeMetadata
				return ut
			},
			wantErr: "database identifier in discover_rds.databases map is required",
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
			wantErr: "database identifier in discover_rds.databases field is required",
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
			wantErr: "database identifier in discover_rds.databases map and field are different",
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
			wantErr: "discovery config in discover_rds.databases field is required",
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
			wantErr: "discovery group in discover_rds.databases field is required",
		},
		{
			name:    "DiscoverAzureVM: valid",
			task:    baseAzureVMDiscoverTask,
			wantErr: noError,
		},
		{
			name: "DiscoverAzureVM: invalid issue type",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseAzureVMDiscoverTask(t)
				ut.Spec.IssueType = "unknown error"
				return ut
			},
			wantErr: "issue_type must be one of",
		},
		{
			name: "DiscoverAzureVM: missing integration",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseAzureVMDiscoverTask(t)
				ut.Spec.Integration = ""
				return ut
			},
			wantErr: "integration cannot be empty",
		},
		{
			name: "DiscoverAzureVM: missing discover azure vm field",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseAzureVMDiscoverTask(t)
				ut.Spec.DiscoverAzureVm = nil
				return ut
			},
			wantErr: "discover_azure_vm field is required",
		},
		{
			name: "DiscoverAzureVM: wrong task name",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseAzureVMDiscoverTask(t)
				ut.Metadata.Name = "another-name"
				return ut
			},
			wantErr: "task name must be d3672afc-63f5-5d8a-bf63-2a2f81d6fa61, got another-name",
		},
		{
			name: "DiscoverAzureVM: missing subscription id",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseAzureVMDiscoverTask(t)
				ut.Spec.DiscoverAzureVm.SubscriptionId = ""
				return ut
			},
			wantErr: "discover_azure_vm.subscription_id field is required",
		},
		{
			name: "DiscoverAzureVM: missing resource group",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseAzureVMDiscoverTask(t)
				ut.Spec.DiscoverAzureVm.ResourceGroup = ""
				return ut
			},
			wantErr: "discover_azure_vm.resource_group field is required",
		},
		{
			name: "DiscoverAzureVM: missing region",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseAzureVMDiscoverTask(t)
				ut.Spec.DiscoverAzureVm.Region = ""
				return ut
			},
			wantErr: "discover_azure_vm.region field is required",
		},
		{
			name: "DiscoverAzureVM: instances - missing vm id in map key",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseAzureVMDiscoverTask(t)
				origVMMetadata := ut.Spec.DiscoverAzureVm.Instances[exampleVMID]
				ut.Spec.DiscoverAzureVm.Instances[""] = origVMMetadata
				return ut
			},
			wantErr: "discover_azure_vm.instances map key is empty",
		},
		{
			name: "DiscoverAzureVM: instances - missing vm id in instance metadata",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseAzureVMDiscoverTask(t)
				origVMMetadata := ut.Spec.DiscoverAzureVm.Instances[exampleVMID]
				origVMMetadata.VmId = ""
				ut.Spec.DiscoverAzureVm.Instances[exampleVMID] = origVMMetadata
				return ut
			},
			wantErr: "discover_azure_vm.instances[/subscriptions/sub-123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachines/my-vm].vm_id field is required",
		},
		{
			name: "DiscoverAzureVM: instances - different vm id",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseAzureVMDiscoverTask(t)
				origVMMetadata := ut.Spec.DiscoverAzureVm.Instances[exampleVMID]
				origVMMetadata.VmId = "/subscriptions/sub-123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachines/other-vm"
				ut.Spec.DiscoverAzureVm.Instances[exampleVMID] = origVMMetadata
				return ut
			},
			wantErr: "discover_azure_vm.instances map key /subscriptions/sub-123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachines/my-vm does not match vm_id /subscriptions/sub-123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachines/other-vm",
		},
		{
			name: "DiscoverAzureVM: instances - missing discovery config",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseAzureVMDiscoverTask(t)
				origVMMetadata := ut.Spec.DiscoverAzureVm.Instances[exampleVMID]
				origVMMetadata.DiscoveryConfig = ""
				ut.Spec.DiscoverAzureVm.Instances[exampleVMID] = origVMMetadata
				return ut
			},
			wantErr: "discover_azure_vm.instances[/subscriptions/sub-123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachines/my-vm].discovery_config field is required",
		},
		{
			name: "DiscoverAzureVM: instances - missing discovery group",
			task: func(t *testing.T) *usertasksv1.UserTask {
				ut := baseAzureVMDiscoverTask(t)
				origVMMetadata := ut.Spec.DiscoverAzureVm.Instances[exampleVMID]
				origVMMetadata.DiscoveryGroup = ""
				ut.Spec.DiscoverAzureVm.Instances[exampleVMID] = origVMMetadata
				return ut
			},
			wantErr: "discover_azure_vm.instances[/subscriptions/sub-123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachines/my-vm].discovery_group field is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUserTask(tt.task(t))
			if tt.wantErr == noError {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
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
		taskOption   []UserTaskOption
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
			taskOption: []UserTaskOption{
				WithExpiration(userTaskExpirationTime),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTask, err := NewDiscoverEC2UserTask(tt.taskSpec, tt.taskOption...)
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
		taskOption   []UserTaskOption
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
			taskOption: []UserTaskOption{
				WithExpiration(userTaskExpirationTime),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTask, err := NewDiscoverEKSUserTask(tt.taskSpec, tt.taskOption...)
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
		taskOption   []UserTaskOption
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
			taskOption: []UserTaskOption{
				WithExpiration(userTaskExpirationTime),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTask, err := NewDiscoverRDSUserTask(tt.taskSpec, tt.taskOption...)
			require.NoError(t, err)
			require.Equal(t, tt.expectedTask, gotTask)
		})
	}
}

func TestNewDiscoverAzureVMUserTask(t *testing.T) {
	t.Parallel()

	userTaskExpirationTime := time.Now()
	userTaskExpirationTimestamp := timestamppb.New(userTaskExpirationTime)
	vmSyncTimestamp := userTaskExpirationTimestamp

	exampleVMID := "/subscriptions/sub-123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachines/my-vm"

	baseAzureVMDiscoverData := &usertasksv1.DiscoverAzureVM{
		SubscriptionId: "sub-123",
		ResourceGroup:  "my-rg",
		Region:         "eastus",
		Instances: map[string]*usertasksv1.DiscoverAzureVMInstance{
			exampleVMID: {
				VmId:            exampleVMID,
				DiscoveryConfig: "dc01",
				DiscoveryGroup:  "dg01",
				SyncTime:        vmSyncTimestamp,
			},
		},
	}

	tests := []struct {
		name         string
		taskGroup    TaskGroup
		expiryTime   time.Time
		data         *usertasksv1.DiscoverAzureVM
		expectedTask *usertasksv1.UserTask
	}{
		{
			name: "valid task created",
			taskGroup: TaskGroup{
				Integration: "my-integration",
				IssueType:   AutoDiscoverAzureVMIssueEnrollmentError,
			},
			expiryTime: userTaskExpirationTime,
			data:       baseAzureVMDiscoverData,
			expectedTask: &usertasksv1.UserTask{
				Kind:    "user_task",
				Version: "v1",
				Metadata: &headerv1.Metadata{
					Name:    "d3672afc-63f5-5d8a-bf63-2a2f81d6fa61",
					Expires: userTaskExpirationTimestamp,
				},
				Spec: &usertasksv1.UserTaskSpec{
					State:    "OPEN",
					TaskType: "discover-azure-vm",

					Integration: "my-integration",
					IssueType:   AutoDiscoverAzureVMIssueEnrollmentError,

					DiscoverAzureVm: baseAzureVMDiscoverData,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTask, err := NewDiscoverAzureVMUserTask(tt.taskGroup, tt.expiryTime, tt.data)
			require.NoError(t, err)
			require.Equal(t, tt.expectedTask, gotTask)
		})
	}
}

func TestTaskNameFromParts(t *testing.T) {
	ns := uuid.MustParse("9d074a38-c369-4cc6-87c7-3eef3156ee23")

	tests := []struct {
		name  string
		parts []string
		want  string
	}{
		{"empty", []string{}, "9995b172-4843-5e9f-ae05-f6ad85e3f509"},
		{"single", []string{"task1"}, "66dab5c1-6995-5773-9e03-7daba7e83635"},
		{"multiple", []string{"a", "b", "c"}, "1c6393ee-bf43-5db8-aeef-0ddd92a591aa"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := taskNameFromParts(ns, tt.parts...)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTaskNameForDiscoverHashStability ensures that hashes generated by
// TaskNameForDiscover* functions remain stable.
func TestTaskNameForDiscoverHashStability(t *testing.T) {
	require.Equal(t, "e3bfba5c-cc27-5f95-9d39-44dad91e1c34",
		TaskNameForDiscoverEC2(TaskNameForDiscoverEC2Parts{
			Integration:     "my-integration",
			IssueType:       "ec2-ssm-invocation-failure",
			AccountID:       "123456789012",
			Region:          "us-east-1",
			SSMDocument:     "my-document",
			InstallerScript: "my-script",
		}))

	require.Equal(t, "681f6f5a-5092-575b-b894-4ba44d685c6b",
		TaskNameForDiscoverEKS(TaskNameForDiscoverEKSParts{
			Integration:     "my-integration",
			IssueType:       "eks-agent-not-connecting",
			AccountID:       "123456789012",
			Region:          "us-east-1",
			AppAutoDiscover: true,
		}))

	require.Equal(t, "8c6014e2-8275-54d7-b285-31e0194b7835",
		TaskNameForDiscoverRDS(TaskNameForDiscoverRDSParts{
			Integration: "my-integration",
			IssueType:   "rds-iam-auth-disabled",
			AccountID:   "123456789012",
			Region:      "us-east-1",
		}))

	require.Equal(t, "d3672afc-63f5-5d8a-bf63-2a2f81d6fa61",
		taskNameForDiscoverAzureVM(
			TaskGroup{
				Integration: "my-integration",
				IssueType:   AutoDiscoverAzureVMIssueEnrollmentError,
			},
			taskNameForDiscoverAzureVMParts{
				SubscriptionID: "sub-123",
				ResourceGroup:  "my-rg",
				Region:         "eastus",
			}))
}
