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

func TestAllDescriptions(t *testing.T) {
	for _, issueType := range usertasks.DiscoverEC2IssueTypes {
		require.NotEmpty(t, usertasks.DescriptionForDiscoverEC2Issue(issueType), "issue type %q is missing descriptions/%s.md file", issueType, issueType)
	}
}
