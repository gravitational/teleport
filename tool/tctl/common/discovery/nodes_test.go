// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package discovery

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	usertaskstypes "github.com/gravitational/teleport/api/types/usertasks"
)

func makeEC2Task(t *testing.T, issueType string, instanceIDs ...string) *usertasksv1.UserTask {
	t.Helper()
	instances := make(map[string]*usertasksv1.DiscoverEC2Instance, len(instanceIDs))
	for _, id := range instanceIDs {
		instances[id] = &usertasksv1.DiscoverEC2Instance{
			InstanceId:      id,
			DiscoveryConfig: "dc01",
			DiscoveryGroup:  "dg01",
		}
	}
	task, err := usertaskstypes.NewDiscoverEC2UserTask(&usertasksv1.UserTaskSpec{
		Integration: "my-integration",
		TaskType:    "discover-ec2",
		IssueType:   issueType,
		State:       "OPEN",
		DiscoverEc2: &usertasksv1.DiscoverEC2{
			AccountId: "111",
			Region:    "us-east-1",
			Instances: instances,
		},
	})
	require.NoError(t, err)
	return task
}

func makeAzureVMTask(t *testing.T, issueType string, vmIDs ...string) *usertasksv1.UserTask {
	t.Helper()
	instances := make(map[string]*usertasksv1.DiscoverAzureVMInstance, len(vmIDs))
	for _, id := range vmIDs {
		instances[id] = &usertasksv1.DiscoverAzureVMInstance{
			VmId:            id,
			DiscoveryConfig: "dc01",
			DiscoveryGroup:  "dg01",
		}
	}
	task, err := usertaskstypes.NewDiscoverAzureVMUserTask(usertaskstypes.TaskGroup{
		Integration: "my-integration",
		IssueType:   issueType,
	}, time.Now().Add(time.Hour), &usertasksv1.DiscoverAzureVM{
		SubscriptionId: "sub-1",
		ResourceGroup:  "rg-1",
		Region:         "eastus",
		Instances:      instances,
	})
	require.NoError(t, err)
	return task
}

func TestCloudNodes(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)

	tests := []struct {
		desc      string
		instances map[string]instanceInfo
		tasks     []*usertasksv1.UserTask
		keyFn     func(*usertasksv1.UserTask) []string
		want      []instanceInfo
	}{
		{
			desc:  "empty input returns empty result",
			keyFn: awsTaskInstanceKeys,
			want:  nil,
		},
		{
			desc: "AWS instance with matching task gets task ID and issue type",
			instances: map[string]instanceInfo{
				"i-aaa": {AWS: &awsInfo{InstanceID: "i-aaa", AccountID: "111"}, Region: "us-east-1"},
			},
			tasks: []*usertasksv1.UserTask{
				makeEC2Task(t, usertaskstypes.AutoDiscoverEC2IssueSSMScriptFailure, "i-aaa"),
			},
			keyFn: awsTaskInstanceKeys,
			want: []instanceInfo{
				{
					AWS:           &awsInfo{InstanceID: "i-aaa", AccountID: "111"},
					Region:        "us-east-1",
					UserTaskID:    "07cccc8f-bb13-5f93-99d8-0ba51ca1da92",
					UserTaskIssue: usertaskstypes.AutoDiscoverEC2IssueSSMScriptFailure,
				},
			},
		},
		{
			desc: "task referencing unknown instance is ignored",
			instances: map[string]instanceInfo{
				"i-aaa": {AWS: &awsInfo{InstanceID: "i-aaa", AccountID: "111"}, Region: "us-east-1"},
			},
			tasks: []*usertasksv1.UserTask{
				makeEC2Task(t, usertaskstypes.AutoDiscoverEC2IssueSSMScriptFailure, "i-other"),
			},
			keyFn: awsTaskInstanceKeys,
			want: []instanceInfo{
				{AWS: &awsInfo{InstanceID: "i-aaa", AccountID: "111"}, Region: "us-east-1"},
			},
		},
		{
			desc: "multiple AWS instances sorted by account, region, descending time",
			instances: map[string]instanceInfo{
				"i-old": {
					AWS: &awsInfo{InstanceID: "i-old", AccountID: "111"}, Region: "us-east-1",
					RunResult: &runResult{Time: now.Add(-time.Hour)},
				},
				"i-new": {
					AWS: &awsInfo{InstanceID: "i-new", AccountID: "111"}, Region: "us-east-1",
					RunResult: &runResult{Time: now},
				},
				"i-other-region": {
					AWS: &awsInfo{InstanceID: "i-other-region", AccountID: "111"}, Region: "us-west-2",
				},
				"i-other-account": {
					AWS: &awsInfo{InstanceID: "i-other-account", AccountID: "222"}, Region: "us-east-1",
				},
			},
			keyFn: awsTaskInstanceKeys,
			want: []instanceInfo{
				{
					AWS: &awsInfo{InstanceID: "i-new", AccountID: "111"}, Region: "us-east-1",
					RunResult: &runResult{Time: now},
				},
				{
					AWS: &awsInfo{InstanceID: "i-old", AccountID: "111"}, Region: "us-east-1",
					RunResult: &runResult{Time: now.Add(-time.Hour)},
				},
				{AWS: &awsInfo{InstanceID: "i-other-region", AccountID: "111"}, Region: "us-west-2"},
				{AWS: &awsInfo{InstanceID: "i-other-account", AccountID: "222"}, Region: "us-east-1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := cloudNodes(tt.instances, tt.tasks, tt.keyFn)
			if tt.want == nil {
				require.Empty(t, got)
			} else {
				require.Equal(t, tt.want, got)
			}
		})
	}
}

func TestInstanceInfo_Status(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		fi   instanceInfo
		want string
	}{
		{
			desc: "no run result, offline",
			fi:   instanceInfo{},
			want: "Unknown",
		},
		{
			desc: "no run result, online",
			fi:   instanceInfo{IsOnline: true},
			want: "Online",
		},
		{
			desc: "success, online",
			fi:   instanceInfo{IsOnline: true, RunResult: &runResult{ExitCode: 0}},
			want: "Online",
		},
		{
			desc: "success but offline",
			fi:   instanceInfo{RunResult: &runResult{ExitCode: 0}},
			want: "Installed (offline)",
		},
		{
			desc: "failure with exit code, offline",
			fi:   instanceInfo{RunResult: &runResult{ExitCode: 1, IsFailure: true}},
			want: "Failed (exit code=1)",
		},
		{
			desc: "failure with exit code, online",
			fi:   instanceInfo{RunResult: &runResult{ExitCode: 1, IsFailure: true}, IsOnline: true},
			want: "Online, exit code=1",
		},
		{
			desc: "API error, offline",
			fi:   instanceInfo{RunResult: &runResult{APIError: "throttled", IsFailure: true}},
			want: "Failed (API error)",
		},
		{
			desc: "API error, online",
			fi:   instanceInfo{RunResult: &runResult{APIError: "throttled", IsFailure: true}, IsOnline: true},
			want: "Online, API error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			require.Equal(t, tt.want, tt.fi.status())
		})
	}
}

func TestInstanceInfo_UserTaskTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		fi   instanceInfo
		want string
	}{
		{
			desc: "empty issue returns empty",
			fi:   instanceInfo{},
			want: "",
		},
		{
			desc: "no cloud info falls back to issue string",
			fi:   instanceInfo{UserTaskIssue: usertaskstypes.AutoDiscoverEC2IssueSSMScriptFailure},
			want: usertaskstypes.AutoDiscoverEC2IssueSSMScriptFailure,
		},
		{
			desc: "AWS known issue resolves to title",
			fi: instanceInfo{
				AWS:           &awsInfo{InstanceID: "i-aaa"},
				UserTaskIssue: usertaskstypes.AutoDiscoverEC2IssueSSMScriptFailure,
			},
			want: "SSM Script failure",
		},
		{
			desc: "AWS unknown issue falls back to issue string",
			fi: instanceInfo{
				AWS:           &awsInfo{InstanceID: "i-aaa"},
				UserTaskIssue: "ec2-not-a-real-issue",
			},
			want: "ec2-not-a-real-issue",
		},
		{
			desc: "Azure known issue resolves to title",
			fi: instanceInfo{
				Azure:         &azureInfo{VMID: "vm-aaa"},
				UserTaskIssue: usertaskstypes.AutoDiscoverAzureVMIssueEnrollmentError,
			},
			want: "Enrollment failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			require.Equal(t, tt.want, tt.fi.userTaskTitle())
		})
	}
}

func TestInstanceInfo_LastTime(t *testing.T) {
	t.Parallel()

	t10 := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	t12 := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	t08 := time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC)

	tests := []struct {
		desc string
		fi   instanceInfo
		want string
	}{
		{
			desc: "run result time",
			fi:   instanceInfo{RunResult: &runResult{Time: t10}},
			want: "2026-01-15T10:00:00Z",
		},
		{
			desc: "no run result with expiry falls back to expiry",
			fi:   instanceInfo{Expiry: t10},
			want: "2026-01-15T10:00:00Z",
		},
		{
			desc: "run result takes precedence over expiry",
			fi:   instanceInfo{RunResult: &runResult{Time: t12}, Expiry: t08},
			want: "2026-01-15T12:00:00Z",
		},
		{
			desc: "no run result and no expiry",
			fi:   instanceInfo{},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			require.Equal(t, tt.want, tt.fi.lastTime())
		})
	}
}

func TestTrimEscape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		in   string
		want string
	}{
		{
			desc: "empty input",
			in:   "",
			want: "",
		},
		{
			desc: "simple output",
			in:   "install failed",
			want: `"install failed"`,
		},
		{
			desc: "newlines escaped",
			in:   "line1\nline2\nline3",
			want: `"line1\nline2\nline3"`,
		},
		{
			desc: "long output truncated",
			in:   "abcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabc",
			want: `"abcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabca..."`,
		},
		{
			desc: "whitespace trimmed",
			in:   "  output  ",
			want: `"output"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			require.Equal(t, tt.want, trimEscape(tt.in))
		})
	}
}

func TestCombineOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc   string
		stdout string
		stderr string
		want   string
	}{
		{
			desc:   "both present joined with newline",
			stdout: "out",
			stderr: "err",
			want:   "out\nerr",
		},
		{
			desc:   "only stdout",
			stdout: "stdout only",
			want:   "stdout only",
		},
		{
			desc:   "only stderr",
			stderr: "stderr only",
			want:   "stderr only",
		},
		{
			desc: "neither",
			want: "",
		},
		{
			desc:   "whitespace-only stderr treated as empty",
			stdout: "real",
			stderr: "   ",
			want:   "real",
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			require.Equal(t, tt.want, combineOutput(tt.stdout, tt.stderr))
		})
	}
}

func TestParseCloudProviders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc    string
		in      string
		want    cloudProviderConfig
		wantErr string
	}{
		{
			desc: "empty enables all",
			in:   "",
			want: cloudProviderConfig{aws: true, azure: true},
		},
		{
			desc: "aws only",
			in:   "aws",
			want: cloudProviderConfig{aws: true},
		},
		{
			desc: "azure only",
			in:   "azure",
			want: cloudProviderConfig{azure: true},
		},
		{
			desc: "both providers",
			in:   "aws,azure",
			want: cloudProviderConfig{aws: true, azure: true},
		},
		{
			desc: "case-insensitive",
			in:   "AWS,Azure",
			want: cloudProviderConfig{aws: true, azure: true},
		},
		{
			desc: "whitespace trimmed",
			in:   " aws , azure ",
			want: cloudProviderConfig{aws: true, azure: true},
		},
		{
			desc: "duplicate providers accepted",
			in:   "aws,aws",
			want: cloudProviderConfig{aws: true},
		},
		{
			desc:    "single comma rejected",
			in:      ",",
			wantErr: "empty cloud provider",
		},
		{
			desc:    "whitespace-only entry rejected",
			in:      "aws, ,azure",
			wantErr: "empty cloud provider",
		},
		{
			desc:    "unknown provider rejected",
			in:      "gcp",
			wantErr: `unknown cloud provider "gcp"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := parseCloudProviders(tt.in)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.Equal(t, cloudProviderConfig{}, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestFilterFailures(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	instances := []instanceInfo{
		{AWS: &awsInfo{InstanceID: "i-ok"}, IsOnline: true, RunResult: &runResult{ExitCode: 0}},
		{AWS: &awsInfo{InstanceID: "i-fail"}, RunResult: &runResult{ExitCode: 1, IsFailure: true, Time: now}},
		{AWS: &awsInfo{InstanceID: "i-task"}, UserTaskID: "some-task-id", UserTaskIssue: "ec2-ssm-script-failure", IsOnline: true},
		{AWS: &awsInfo{InstanceID: "i-online"}, IsOnline: true},
	}
	got := filterFailures(instances)
	require.Len(t, got, 2)
	require.Equal(t, "i-fail", got[0].AWS.InstanceID)
	require.Equal(t, "i-task", got[1].AWS.InstanceID)
}
