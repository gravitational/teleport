/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package discovery

import (
	"context"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/usertasks"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestTruncateErrorMessage(t *testing.T) {
	for _, tt := range []struct {
		name     string
		in       discoveryconfig.Status
		expected *string
	}{
		{
			name:     "nil error message",
			in:       discoveryconfig.Status{},
			expected: nil,
		},
		{
			name:     "small error messages are not changed",
			in:       discoveryconfig.Status{ErrorMessage: stringPointer("small error message")},
			expected: stringPointer("small error message"),
		},
		{
			name:     "large error messages are truncated",
			in:       discoveryconfig.Status{ErrorMessage: stringPointer(strings.Repeat("A", 1024*100+1))},
			expected: stringPointer(strings.Repeat("A", 1024*100)),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateErrorMessage(tt.in)
			require.Equal(t, tt.expected, got)
		})
	}
}

type mockInstance struct {
	syncTime       *timestamppb.Timestamp
	discoveryGroup string
}

func (m *mockInstance) GetSyncTime() *timestamppb.Timestamp {
	return m.syncTime
}

func (m *mockInstance) GetDiscoveryGroup() string {
	return m.discoveryGroup
}

func TestMergeExistingInstances(t *testing.T) {
	s, _ := newTaskUpdater(t)
	clock := s.clock
	pollInterval := s.PollInterval

	now := clock.Now()
	tooOld := now.Add(-3 * pollInterval)
	recent := now.Add(-pollInterval)

	tests := []struct {
		name           string
		oldInstances   map[string]*mockInstance
		freshInstances map[string]*mockInstance
		expected       map[string]*mockInstance
	}{
		{
			name: "skip instances from the same discovery group",
			oldInstances: map[string]*mockInstance{
				"inst-1": {
					syncTime:       timestamppb.New(recent),
					discoveryGroup: "group-1",
				},
			},
			freshInstances: map[string]*mockInstance{},
			expected:       map[string]*mockInstance{},
		},
		{
			name: "skip expired instances",
			oldInstances: map[string]*mockInstance{
				"inst-2": {
					syncTime:       timestamppb.New(tooOld),
					discoveryGroup: "group-2",
				},
			},
			freshInstances: map[string]*mockInstance{},
			expected:       map[string]*mockInstance{},
		},
		{
			name: "merge missing instances",
			oldInstances: map[string]*mockInstance{
				"inst-3": {
					syncTime:       timestamppb.New(recent),
					discoveryGroup: "group-2",
				},
			},
			freshInstances: map[string]*mockInstance{},
			expected: map[string]*mockInstance{
				"inst-3": {
					syncTime:       timestamppb.New(recent),
					discoveryGroup: "group-2",
				},
			},
		},
		{
			name: "do not overwrite fresh instances",
			oldInstances: map[string]*mockInstance{
				"inst-4": {
					syncTime:       timestamppb.New(recent),
					discoveryGroup: "group-2",
				},
			},
			freshInstances: map[string]*mockInstance{
				"inst-4": {
					syncTime:       timestamppb.New(now),
					discoveryGroup: "group-1",
				},
			},
			expected: map[string]*mockInstance{
				"inst-4": {
					syncTime:       timestamppb.New(now),
					discoveryGroup: "group-1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workingCopy := maps.Clone(tt.freshInstances)
			mergeExistingInstances(s, tt.oldInstances, workingCopy)
			require.Equal(t, tt.expected, workingCopy)
		})
	}
}

func TestMergeUpsertUserTask(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	syncTime := timestamppb.New(clock.Now())

	newTask := func(t *testing.T, tag string) *usertasksv1.UserTask {
		ut, err := usertasks.NewDiscoverAzureVMUserTask(
			usertasks.TaskGroup{
				Integration: "my-int",
				IssueType:   usertasks.AutoDiscoverAzureVMIssueEnrollmentError,
			},
			clock.Now().Add(20*time.Minute),
			&usertasksv1.DiscoverAzureVM{
				Instances: map[string]*usertasksv1.DiscoverAzureVMInstance{
					tag: {
						VmId:            tag,
						DiscoveryConfig: tag,
						DiscoveryGroup:  tag,
						SyncTime:        syncTime,
					},
				},
				// these feed into task name, in addition to the task group above.
				SubscriptionId: "sub-123",
				ResourceGroup:  "rg-123",
				Region:         "westus",
			},
		)
		require.NoError(t, err)
		return ut
	}

	tests := []struct {
		name         string
		existingTask *usertasksv1.UserTask
		newTask      *usertasksv1.UserTask
		mergeCalled  bool
	}{
		{
			name:         "no existing task - merge not called",
			existingTask: nil,
			newTask:      newTask(t, "foo"),
			mergeCalled:  false,
		},
		{
			name:         "existing task with spec - merge is called",
			existingTask: newTask(t, "bar"),
			newTask:      newTask(t, "foo"),
			mergeCalled:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, ap := newTaskUpdater(t, tt.existingTask)

			mergeCalled := false
			mergeFunc := func(oldSpec *usertasksv1.UserTaskSpec, newSpec *usertasksv1.UserTaskSpec) {
				mergeCalled = true
			}

			require.NoError(t, s.mergeUpsertUserTask(tt.newTask, mergeFunc))
			require.Equal(t, tt.mergeCalled, mergeCalled, "mergeCalled mismatch")

			// Verify the task was upserted
			upsertedTask, err := ap.GetUserTask(s.ctx, tt.newTask.GetMetadata().GetName())
			require.NoError(t, err)
			require.NotNil(t, upsertedTask)
			require.Empty(t, cmp.Diff(tt.newTask.Spec, upsertedTask.Spec, protocmp.Transform()))
		})
	}
}

type mocktaskUpdaterAccessPoint struct {
	types.Semaphores

	mu sync.Mutex

	tasks map[string]*usertasksv1.UserTask
}

func (m *mocktaskUpdaterAccessPoint) AcquireSemaphore(ctx context.Context, params types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return &types.SemaphoreLease{}, nil
}

func (m *mocktaskUpdaterAccessPoint) CancelSemaphoreLease(ctx context.Context, lease types.SemaphoreLease) error {
	return nil
}

func (m *mocktaskUpdaterAccessPoint) UpsertUserTask(ctx context.Context, req *usertasksv1.UserTask) (*usertasksv1.UserTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tasks[req.GetMetadata().GetName()] = req

	return req, nil
}

func (m *mocktaskUpdaterAccessPoint) GetUserTask(ctx context.Context, name string) (*usertasksv1.UserTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[name]
	if !ok {
		return nil, trace.NotFound("task %q not found", name)
	}
	return task, nil
}

func newTaskUpdater(t *testing.T, existingTasks ...*usertasksv1.UserTask) (*taskUpdater, *mocktaskUpdaterAccessPoint) {
	t.Helper()

	clock := clockwork.NewFakeClock()

	ap := &mocktaskUpdaterAccessPoint{
		tasks: make(map[string]*usertasksv1.UserTask),
	}

	manager := &taskUpdater{
		ctx:   t.Context(),
		clock: clock,

		DiscoveryGroup: "group-1",
		ServerID:       "discover-server-id",
		PollInterval:   10 * time.Minute,
		Log:            logtest.NewLogger(),
		AccessPoint:    ap,
	}

	for _, task := range existingTasks {
		if task == nil {
			continue
		}
		_, err := manager.AccessPoint.UpsertUserTask(manager.ctx, task)
		require.NoError(t, err)
	}

	return manager, ap
}

func TestAzureVMTasks_AddFailedEnrollment(t *testing.T) {
	t.Parallel()

	var testTaskGroup = usertasks.TaskGroup{Integration: "my-int", IssueType: usertasks.AutoDiscoverAzureVMIssueEnrollmentError}
	var testTaskGroupAlt = usertasks.TaskGroup{Integration: "my-int", IssueType: usertasks.AutoDiscoverAzureVMIssueVMNotRunning}
	var testAzureKey = azureVMTaskKey{subscriptionID: "sub-1", resourceGroup: "rg-1", region: "westus"}
	var testAzureKeyAlt = azureVMTaskKey{subscriptionID: "sub-2", resourceGroup: "rg-2", region: "eastus"}

	syncTime := timestamppb.New(time.Now())

	vm := func(tag string) *usertasksv1.DiscoverAzureVMInstance {
		return &usertasksv1.DiscoverAzureVMInstance{
			VmId:            tag,
			DiscoveryConfig: "dc-01",
			DiscoveryGroup:  "group-1",
			SyncTime:        syncTime,
		}
	}

	azureData := func(key azureVMTaskKey, instances ...string) *usertasksv1.DiscoverAzureVM {
		data := &usertasksv1.DiscoverAzureVM{
			SubscriptionId: key.subscriptionID,
			ResourceGroup:  key.resourceGroup,
			Region:         key.region,
			Instances:      make(map[string]*usertasksv1.DiscoverAzureVMInstance),
		}
		for _, instance := range instances {
			data.Instances[instance] = vm(instance)
		}
		return data
	}

	tests := []struct {
		name     string
		mutate   func(tasks *azureVMTasks)
		expected map[usertasks.TaskGroup]map[azureVMTaskKey]*usertasksv1.DiscoverAzureVM
	}{
		{
			name: "empty integration is ignored",
			mutate: func(tasks *azureVMTasks) {
				tasks.addFailedEnrollment(usertasks.TaskGroup{Integration: "", IssueType: "x"}, testAzureKey, vm("foo"))
			},
		},
		{
			name: "empty issue type is ignored",
			mutate: func(tasks *azureVMTasks) {
				tasks.addFailedEnrollment(usertasks.TaskGroup{Integration: "x", IssueType: ""}, testAzureKey, vm("foo"))
			},
		},
		{
			name: "creates task group and adds VM",
			mutate: func(tasks *azureVMTasks) {
				tasks.addFailedEnrollment(testTaskGroup, testAzureKey, vm("foo"))
			},
			expected: map[usertasks.TaskGroup]map[azureVMTaskKey]*usertasksv1.DiscoverAzureVM{
				testTaskGroup: {
					testAzureKey: azureData(testAzureKey, "foo"),
				},
			},
		},
		{
			name: "adds multiple VMs to same key",
			mutate: func(tasks *azureVMTasks) {
				tasks.addFailedEnrollment(testTaskGroup, testAzureKey, vm("foo"))
				tasks.addFailedEnrollment(testTaskGroup, testAzureKey, vm("bar"))
			},
			expected: map[usertasks.TaskGroup]map[azureVMTaskKey]*usertasksv1.DiscoverAzureVM{
				testTaskGroup: {
					testAzureKey: azureData(testAzureKey, "foo", "bar"),
				},
			},
		},
		{
			name: "different issue types create separate groups",
			mutate: func(tasks *azureVMTasks) {
				tasks.addFailedEnrollment(testTaskGroup, testAzureKey, vm("foo"))
				tasks.addFailedEnrollment(testTaskGroupAlt, testAzureKey, vm("bar"))
			},
			expected: map[usertasks.TaskGroup]map[azureVMTaskKey]*usertasksv1.DiscoverAzureVM{
				testTaskGroup: {
					testAzureKey: azureData(testAzureKey, "foo"),
				},
				testTaskGroupAlt: {
					testAzureKey: azureData(testAzureKey, "bar"),
				},
			},
		},
		{
			name: "different azure keys create separate entries",
			mutate: func(tasks *azureVMTasks) {
				tasks.addFailedEnrollment(testTaskGroup, testAzureKey, vm("foo"))
				tasks.addFailedEnrollment(testTaskGroup, testAzureKeyAlt, vm("bar"))
			},
			expected: map[usertasks.TaskGroup]map[azureVMTaskKey]*usertasksv1.DiscoverAzureVM{
				testTaskGroup: {
					testAzureKey:    azureData(testAzureKey, "foo"),
					testAzureKeyAlt: azureData(testAzureKeyAlt, "bar"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := &azureVMTasks{}
			tt.mutate(tasks)
			require.Empty(t, cmp.Diff(tt.expected, tasks.taskGroups, protocmp.Transform()))
		})
	}
}

func TestAWSEC2Tasks_AddFailedEnrollment(t *testing.T) {
	t.Parallel()

	var testEC2Key = awsEC2TaskKey{
		integration:     "my-int",
		issueType:       usertasks.AutoDiscoverEC2IssueSSMScriptFailure,
		accountID:       "123456789012",
		region:          "us-west-2",
		ssmDocument:     "doc",
		installerScript: "script",
	}

	var testEC2KeyAlt = awsEC2TaskKey{
		integration:     "my-int",
		issueType:       usertasks.AutoDiscoverEC2IssueSSMScriptFailure,
		accountID:       "123456789012",
		region:          "us-east-1",
		ssmDocument:     "doc",
		installerScript: "script",
	}

	var testEC2KeyPermIssue = awsEC2TaskKey{
		integration:     "my-int",
		issueType:       usertasks.AutoDiscoverEC2IssuePermAccountDenied,
		accountID:       "",
		region:          "",
		ssmDocument:     "doc",
		installerScript: "script",
	}

	syncTime := timestamppb.New(time.Now())

	instance := func(id string) *usertasksv1.DiscoverEC2Instance {
		return &usertasksv1.DiscoverEC2Instance{
			InstanceId:      id,
			DiscoveryConfig: "dc-01",
			DiscoveryGroup:  "group-1",
			SyncTime:        syncTime,
		}
	}

	ec2Data := func(key awsEC2TaskKey, instances ...string) *usertasksv1.DiscoverEC2 {
		data := &usertasksv1.DiscoverEC2{
			AccountId:       key.accountID,
			Region:          key.region,
			SsmDocument:     key.ssmDocument,
			InstallerScript: key.installerScript,
			Instances:       make(map[string]*usertasksv1.DiscoverEC2Instance),
		}
		for _, inst := range instances {
			data.Instances[inst] = instance(inst)
		}
		return data
	}

	tests := []struct {
		name     string
		mutate   func(tasks *awsEC2Tasks)
		expected map[awsEC2TaskKey]*usertasksv1.DiscoverEC2
	}{
		{
			name: "empty integration is ignored",
			mutate: func(tasks *awsEC2Tasks) {
				key := testEC2Key
				key.integration = ""
				tasks.addFailedEnrollment(key, instance("i-1"))
			},
		},
		{
			name: "empty issue type is ignored",
			mutate: func(tasks *awsEC2Tasks) {
				key := testEC2Key
				key.issueType = ""
				tasks.addFailedEnrollment(key, instance("i-1"))
			},
		},
		{
			name: "creates task entry and adds instance",
			mutate: func(tasks *awsEC2Tasks) {
				tasks.addFailedEnrollment(testEC2Key, instance("i-1"))
			},
			expected: map[awsEC2TaskKey]*usertasksv1.DiscoverEC2{
				testEC2Key: ec2Data(testEC2Key, "i-1"),
			},
		},
		{
			name: "adds multiple instances to same key",
			mutate: func(tasks *awsEC2Tasks) {
				tasks.addFailedEnrollment(testEC2Key, instance("i-1"))
				tasks.addFailedEnrollment(testEC2Key, instance("i-2"))
			},
			expected: map[awsEC2TaskKey]*usertasksv1.DiscoverEC2{
				testEC2Key: ec2Data(testEC2Key, "i-1", "i-2"),
			},
		},
		{
			name: "different keys create separate entries",
			mutate: func(tasks *awsEC2Tasks) {
				tasks.addFailedEnrollment(testEC2Key, instance("i-1"))
				tasks.addFailedEnrollment(testEC2KeyAlt, instance("i-2"))
			},
			expected: map[awsEC2TaskKey]*usertasksv1.DiscoverEC2{
				testEC2Key:    ec2Data(testEC2Key, "i-1"),
				testEC2KeyAlt: ec2Data(testEC2KeyAlt, "i-2"),
			},
		},
		{
			name: "nil instance creates task entry without instances (permission issues)",
			mutate: func(tasks *awsEC2Tasks) {
				tasks.addFailedEnrollment(testEC2KeyPermIssue, nil)
			},
			expected: map[awsEC2TaskKey]*usertasksv1.DiscoverEC2{
				testEC2KeyPermIssue: ec2Data(testEC2KeyPermIssue),
			},
		},
		{
			name: "nil instance with non-permission issue still creates entry",
			mutate: func(tasks *awsEC2Tasks) {
				tasks.addFailedEnrollment(testEC2Key, nil)
			},
			expected: map[awsEC2TaskKey]*usertasksv1.DiscoverEC2{
				testEC2Key: ec2Data(testEC2Key),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := &awsEC2Tasks{}
			tt.mutate(tasks)
			require.Empty(t, cmp.Diff(tt.expected, tasks.instancesIssues, protocmp.Transform()))
		})
	}
}

func TestAzureVMTasks_UpsertAll(t *testing.T) {
	t.Parallel()

	var testTaskGroup = usertasks.TaskGroup{Integration: "my-int", IssueType: usertasks.AutoDiscoverAzureVMIssueEnrollmentError}
	var testAzureKey = azureVMTaskKey{subscriptionID: "sub-1", resourceGroup: "rg-1", region: "westus"}
	var testAzureKeyAlt = azureVMTaskKey{subscriptionID: "sub-2", resourceGroup: "rg-2", region: "eastus"}

	azureData := func(key azureVMTaskKey, instances ...string) *usertasksv1.DiscoverAzureVM {
		data := &usertasksv1.DiscoverAzureVM{
			SubscriptionId: key.subscriptionID,
			ResourceGroup:  key.resourceGroup,
			Region:         key.region,
			Instances:      make(map[string]*usertasksv1.DiscoverAzureVMInstance),
		}
		for _, instance := range instances {
			data.Instances[instance] = &usertasksv1.DiscoverAzureVMInstance{
				VmId:            instance,
				DiscoveryConfig: "dc-01",
				DiscoveryGroup:  "group-1",
				SyncTime:        timestamppb.New(time.Now()),
			}
		}
		return data
	}

	tests := []struct {
		name          string
		tasks         *azureVMTasks
		existingTasks []*usertasksv1.UserTask
		expectedTasks int
	}{
		{
			name:          "nil taskGroups does not panic",
			tasks:         &azureVMTasks{},
			expectedTasks: 0,
		},
		{
			name: "empty instances are skipped",
			tasks: &azureVMTasks{
				taskGroups: map[usertasks.TaskGroup]map[azureVMTaskKey]*usertasksv1.DiscoverAzureVM{
					testTaskGroup: {testAzureKey: azureData(testAzureKey)},
				},
			},
			expectedTasks: 0,
		},
		{
			name: "upserts single task",
			tasks: &azureVMTasks{
				taskGroups: map[usertasks.TaskGroup]map[azureVMTaskKey]*usertasksv1.DiscoverAzureVM{
					testTaskGroup: {
						testAzureKey: azureData(testAzureKey, "foo"),
					},
				},
			},
			expectedTasks: 1,
		},
		{
			name: "upserts multiple tasks for different keys",
			tasks: &azureVMTasks{
				taskGroups: map[usertasks.TaskGroup]map[azureVMTaskKey]*usertasksv1.DiscoverAzureVM{
					testTaskGroup: {
						testAzureKey:    azureData(testAzureKey, "foo"),
						testAzureKeyAlt: azureData(testAzureKeyAlt, "bar"),
					},
				},
			},
			expectedTasks: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, ap := newTaskUpdater(t, tt.existingTasks...)

			tt.tasks.upsertAll(s)

			tasks := slices.Collect(maps.Values(ap.tasks))
			require.Len(t, tasks, tt.expectedTasks)
		})
	}
}
