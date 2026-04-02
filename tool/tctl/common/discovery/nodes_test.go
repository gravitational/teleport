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

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	usertaskstypes "github.com/gravitational/teleport/api/types/usertasks"
)

func TestCorrelate(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	tests := []struct {
		desc      string
		ssmEvents []*apievents.SSMRun
		nodes     []types.Server
		want      []instanceInfo
	}{
		{
			desc: "empty input returns empty result",
			want: nil,
		},
		{
			desc: "online marking from matching nodes",
			ssmEvents: []*apievents.SSMRun{
				makeSSMRun("i-aaa", "111", "us-east-1", "Failed", 1, "err", now),
			},
			nodes: []types.Server{makeNode("node-1", "i-aaa", "111", "us-east-1", time.Time{})},
			want: []instanceInfo{
				{
					AWS:       &awsInfo{InstanceID: "i-aaa", AccountID: "111"},
					Region:    "us-east-1",
					IsOnline:  true,
					RunResult: &runResult{ExitCode: 1, Output: "err", Time: now, IsFailure: true},
				},
			},
		},
		{
			desc:  "online node without events enriches region and account",
			nodes: []types.Server{makeNode("node-1", "i-aaa", "111", "us-east-1", now.Add(10*time.Minute))},
			want: []instanceInfo{
				{
					AWS:      &awsInfo{InstanceID: "i-aaa", AccountID: "111"},
					Region:   "us-east-1",
					IsOnline: true,
					Expiry:   now.Add(10 * time.Minute),
				},
			},
		},
		{
			desc: "node enriches account for event-only instance",
			ssmEvents: []*apievents.SSMRun{
				makeSSMRun("i-aaa", "", "us-east-1", "Failed", 1, "err", now),
			},
			nodes: []types.Server{makeNode("node-1", "i-aaa", "111", "", time.Time{})},
			want: []instanceInfo{
				{
					AWS:       &awsInfo{InstanceID: "i-aaa", AccountID: "111"},
					Region:    "us-east-1",
					IsOnline:  true,
					RunResult: &runResult{ExitCode: 1, Output: "err", Time: now, IsFailure: true},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := correlate(tt.ssmEvents, tt.nodes)
			if tt.want == nil {
				require.Empty(t, got)
			} else {
				require.Equal(t, tt.want, got)
			}
		})
	}
}

func TestInstanceInfo_Status(t *testing.T) {
	tests := []struct {
		desc string
		fi   instanceInfo
		want string
	}{
		{
			desc: "no run result, offline",
			fi:   instanceInfo{},
			want: "",
		},
		{
			desc: "no run result, online",
			fi:   instanceInfo{IsOnline: true},
			want: "Online",
		},
		{
			desc: "success and online",
			fi:   instanceInfo{RunResult: &runResult{ExitCode: 0}, IsOnline: true},
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
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			require.Equal(t, tt.want, tt.fi.status())
		})
	}
}

func TestInstanceInfo_LastTime(t *testing.T) {
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

func TestInstanceInfo_RunOutput(t *testing.T) {
	tests := []struct {
		desc string
		fi   instanceInfo
		want string
	}{
		{
			desc: "no run result",
			fi:   instanceInfo{},
			want: "",
		},
		{
			desc: "simple output",
			fi:   instanceInfo{RunResult: &runResult{Output: "install failed"}},
			want: `"install failed"`,
		},
		{
			desc: "newlines escaped",
			fi:   instanceInfo{RunResult: &runResult{Output: "line1\nline2\nline3"}},
			want: `"line1\nline2\nline3"`,
		},
		{
			desc: "carriage returns escaped",
			fi:   instanceInfo{RunResult: &runResult{Output: "line1\r\nline2"}},
			want: `"line1\r\nline2"`,
		},
		{
			desc: "tabs escaped",
			fi:   instanceInfo{RunResult: &runResult{Output: "col1\tcol2"}},
			want: `"col1\tcol2"`,
		},
		{
			desc: "whitespace trimmed",
			fi:   instanceInfo{RunResult: &runResult{Output: "  output  "}},
			want: `"output"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			require.Equal(t, tt.want, tt.fi.runOutput())
		})
	}
}

func TestCombineOutput(t *testing.T) {
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

// TestMatchUserTasks verifies that matchUserTasks correctly populates the
// UserTask field on instances by looking up their cloud instance ID in the
// DiscoverEC2 instance maps carried by user tasks. It checks that:
//   - known issue types resolve to human-readable titles (from embedded descriptions)
//   - unknown issue types fall back to the raw issue type string
//   - instances not mentioned in any task remain unchanged
//   - instances without cloud metadata are safely skipped
//   - tasks with nil specs are safely skipped
//   - multiple tasks can match different instances independently
func TestMatchUserTasks(t *testing.T) {
	makeTask := func(t *testing.T, issueType string, instanceIDs ...string) *usertasksv1.UserTask {
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

	tests := []struct {
		desc      string
		instances []instanceInfo
		tasks     []*usertasksv1.UserTask
		wantTasks map[string]string // instance ID -> expected UserTask value
	}{
		{
			desc: "known issue type populates title from description",
			instances: []instanceInfo{
				{AWS: &awsInfo{InstanceID: "i-aaa", AccountID: "111"}},
			},
			tasks: []*usertasksv1.UserTask{
				makeTask(t, usertaskstypes.AutoDiscoverEC2IssueSSMScriptFailure, "i-aaa"),
			},
			wantTasks: map[string]string{
				"i-aaa": "SSM Script failure",
			},
		},
		{
			desc: "unknown issue type falls back to raw issue type string",
			instances: []instanceInfo{
				{AWS: &awsInfo{InstanceID: "i-bbb", AccountID: "111"}},
			},
			tasks: []*usertasksv1.UserTask{
				{Spec: &usertasksv1.UserTaskSpec{
					IssueType: "some-unknown-issue",
					DiscoverEc2: &usertasksv1.DiscoverEC2{
						Instances: map[string]*usertasksv1.DiscoverEC2Instance{
							"i-bbb": {InstanceId: "i-bbb"},
						},
					},
				}},
			},
			wantTasks: map[string]string{
				"i-bbb": "some-unknown-issue",
			},
		},
		{
			desc: "non-matching instance is unchanged",
			instances: []instanceInfo{
				{AWS: &awsInfo{InstanceID: "i-other", AccountID: "111"}},
			},
			tasks: []*usertasksv1.UserTask{
				makeTask(t, usertaskstypes.AutoDiscoverEC2IssueSSMScriptFailure, "i-aaa"),
			},
			wantTasks: map[string]string{
				"i-other": "",
			},
		},
		{
			desc: "instance without cloud info is skipped",
			instances: []instanceInfo{
				{Region: "us-east-1"},
			},
			tasks: []*usertasksv1.UserTask{
				makeTask(t, usertaskstypes.AutoDiscoverEC2IssueSSMScriptFailure, "i-aaa"),
			},
			wantTasks: map[string]string{},
		},
		{
			desc: "task with nil spec is skipped",
			instances: []instanceInfo{
				{AWS: &awsInfo{InstanceID: "i-aaa", AccountID: "111"}},
			},
			tasks: []*usertasksv1.UserTask{
				{Spec: nil},
			},
			wantTasks: map[string]string{
				"i-aaa": "",
			},
		},
		{
			desc: "multiple tasks match multiple instances",
			instances: []instanceInfo{
				{AWS: &awsInfo{InstanceID: "i-aaa", AccountID: "111"}},
				{AWS: &awsInfo{InstanceID: "i-bbb", AccountID: "111"}},
				{AWS: &awsInfo{InstanceID: "i-ccc", AccountID: "111"}},
			},
			tasks: []*usertasksv1.UserTask{
				makeTask(t, usertaskstypes.AutoDiscoverEC2IssueSSMScriptFailure, "i-aaa"),
				makeTask(t, usertaskstypes.AutoDiscoverEC2IssueSSMInstanceNotRegistered, "i-bbb"),
			},
			wantTasks: map[string]string{
				"i-aaa": "SSM Script failure",
				"i-bbb": "SSM Agent not registered",
				"i-ccc": "",
			},
		},
		{
			desc:      "empty tasks and instances",
			instances: nil,
			tasks:     nil,
			wantTasks: map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			matchUserTasks(tt.instances, tt.tasks)
			for _, inst := range tt.instances {
				ci := inst.cloud()
				if ci == nil {
					continue
				}
				want := tt.wantTasks[ci.cloudInstanceID()]
				require.Equal(t, want, inst.UserTask, "instance %s", ci.cloudInstanceID())
			}
		})
	}
}

func TestFilterFailures(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	instances := []instanceInfo{
		{AWS: &awsInfo{InstanceID: "i-ok"}, IsOnline: true, RunResult: &runResult{ExitCode: 0}},
		{AWS: &awsInfo{InstanceID: "i-fail"}, RunResult: &runResult{ExitCode: 1, IsFailure: true, Time: now}},
		{AWS: &awsInfo{InstanceID: "i-task"}, UserTask: "Some issue", IsOnline: true},
		{AWS: &awsInfo{InstanceID: "i-online"}, IsOnline: true},
	}
	got := filterFailures(instances)
	require.Len(t, got, 2)
	require.Equal(t, "i-fail", got[0].AWS.InstanceID)
	require.Equal(t, "i-task", got[1].AWS.InstanceID)
}

func TestResolveTimeRange(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	clock := clockwork.NewFakeClockAt(now)

	tests := []struct {
		desc     string
		input    string
		wantErr  bool
		wantFrom time.Time
		wantTo   time.Time
	}{
		{
			desc:     "valid duration 30m",
			input:    "30m",
			wantFrom: now.Add(-30 * time.Minute),
			wantTo:   now,
		},
		{
			desc:     "valid duration 2h",
			input:    "2h",
			wantFrom: now.Add(-2 * time.Hour),
			wantTo:   now,
		},
		{
			desc:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			desc:    "invalid input",
			input:   "notaduration",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			from, to, err := resolveTimeRange(clock, tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantFrom, from)
			require.Equal(t, tt.wantTo, to)
		})
	}
}
