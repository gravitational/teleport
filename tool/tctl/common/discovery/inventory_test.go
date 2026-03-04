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
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// makeSSMRun creates an *apievents.SSMRun suitable for use in tests.
// Sets the event code based on status: "Success" → SSMRunSuccessCode, else SSMRunFailCode.
func makeSSMRun(instanceID, accountID, region, status string, exitCode int64, output string, ts time.Time) *apievents.SSMRun {
	code := libevents.SSMRunFailCode
	if strings.EqualFold(status, "Success") {
		code = libevents.SSMRunSuccessCode
	}
	return &apievents.SSMRun{
		Metadata: apievents.Metadata{
			ID:   instanceID,
			Type: libevents.SSMRunEvent,
			Time: ts,
			Code: code,
		},
		InstanceID:     instanceID,
		AccountID:      accountID,
		Region:         region,
		Status:         status,
		ExitCode:       exitCode,
		StandardOutput: output,
	}
}

// makeNode creates a types.Server with the given name, AWS instance ID, account, region, and expiry.
func makeNode(name, awsInstanceID, accountID, region string, expiry time.Time) types.Server {
	node := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: name,
			Labels: map[string]string{
				types.AWSInstanceIDLabel: awsInstanceID,
			},
		},
	}
	if accountID != "" || region != "" {
		node.Spec.CloudMetadata = &types.CloudMetadata{
			AWS: &types.AWSInfo{
				AccountID:  accountID,
				InstanceID: awsInstanceID,
				Region:     region,
			},
		}
	}
	if !expiry.IsZero() {
		node.Metadata.SetExpiry(expiry)
	}
	return node
}

// ---------------------------------------------------------------------------
// correlate tests
// ---------------------------------------------------------------------------

func TestCorrelate(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	early := now.Add(-time.Minute)

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
			desc: "SSM failures and successes both included",
			ssmEvents: []*apievents.SSMRun{
				makeSSMRun("i-aaa", "111", "us-east-1", "Failed", 1, "install failed", now),
				makeSSMRun("i-bbb", "222", "us-west-2", "Failed", 2, "timeout", now.Add(-time.Minute)),
				makeSSMRun("i-ccc", "333", "eu-west-1", "Success", 0, "", now),
			},
			want: []instanceInfo{
				{
					InstanceID: "i-aaa",
					Region:     "us-east-1",
					AccountID:  "111",
					SSMResult:  &ssmResult{ExitCode: 1, Output: "install failed", Time: now, IsFailure: true},
				},
				{
					InstanceID: "i-bbb",
					Region:     "us-west-2",
					AccountID:  "222",
					SSMResult:  &ssmResult{ExitCode: 2, Output: "timeout", Time: early, IsFailure: true},
				},
				{
					InstanceID: "i-ccc",
					Region:     "eu-west-1",
					AccountID:  "333",
					SSMResult:  &ssmResult{ExitCode: 0, Time: now},
				},
			},
		},
		{
			desc: "online marking from matching nodes",
			ssmEvents: []*apievents.SSMRun{
				makeSSMRun("i-aaa", "111", "us-east-1", "Failed", 1, "err", now),
			},
			nodes: []types.Server{makeNode("node-1", "i-aaa", "111", "us-east-1", time.Time{})},
			want: []instanceInfo{
				{
					InstanceID: "i-aaa",
					Region:     "us-east-1",
					AccountID:  "111",
					IsOnline:   true,
					SSMResult:  &ssmResult{ExitCode: 1, Output: "err", Time: now, IsFailure: true},
				},
			},
		},
		{
			desc: "SSM event with empty instance ID is skipped",
			ssmEvents: []*apievents.SSMRun{
				{
					Metadata:   apievents.Metadata{Time: now, Code: libevents.SSMRunFailCode},
					InstanceID: "",
					Status:     "Failed",
					ExitCode:   1,
				},
			},
			want: nil,
		},
		{
			desc:  "online node without events enriches region and account",
			nodes: []types.Server{makeNode("node-1", "i-aaa", "111", "us-east-1", now.Add(10*time.Minute))},
			want: []instanceInfo{
				{
					InstanceID: "i-aaa",
					Region:     "us-east-1",
					AccountID:  "111",
					IsOnline:   true,
					Expiry:     now.Add(10 * time.Minute),
				},
			},
		},
		{
			desc: "dedup keeps most recent SSM failure",
			ssmEvents: []*apievents.SSMRun{
				makeSSMRun("i-aaa", "111", "us-east-1", "Failed", 1, "newest error", now),
				makeSSMRun("i-aaa", "111", "us-east-1", "Failed", 2, "older error", now.Add(-10*time.Minute)),
			},
			want: []instanceInfo{
				{
					InstanceID: "i-aaa",
					Region:     "us-east-1",
					AccountID:  "111",
					SSMResult:  &ssmResult{ExitCode: 1, Output: "newest error", Time: now, IsFailure: true},
				},
			},
		},
		{
			desc: "node enriches account for SSM-only instance",
			ssmEvents: []*apievents.SSMRun{
				makeSSMRun("i-aaa", "", "us-east-1", "Failed", 1, "err", now),
			},
			nodes: []types.Server{makeNode("node-1", "i-aaa", "111", "", time.Time{})},
			want: []instanceInfo{
				{
					InstanceID: "i-aaa",
					Region:     "us-east-1",
					AccountID:  "111",
					IsOnline:   true,
					SSMResult:  &ssmResult{ExitCode: 1, Output: "err", Time: now, IsFailure: true},
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

// ---------------------------------------------------------------------------
// instanceInfo method tests
// ---------------------------------------------------------------------------

func TestInstanceInfo_Result(t *testing.T) {
	tests := []struct {
		desc string
		fi   instanceInfo
		want string
	}{
		{
			desc: "no SSM result",
			fi:   instanceInfo{},
			want: "",
		},
		{
			desc: "failure with output",
			fi:   instanceInfo{SSMResult: &ssmResult{ExitCode: 1, Output: "install failed"}},
			want: "exit=1",
		},
		{
			desc: "failure without output",
			fi:   instanceInfo{SSMResult: &ssmResult{ExitCode: 2}},
			want: "exit=2",
		},
		{
			desc: "success",
			fi:   instanceInfo{SSMResult: &ssmResult{ExitCode: 0}},
			want: "exit=0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			require.Equal(t, tt.want, tt.fi.result())
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
			desc: "SSM failure time",
			fi:   instanceInfo{SSMResult: &ssmResult{Time: t10}},
			want: "2026-01-15T10:00:00Z",
		},
		{
			desc: "no failure with expiry falls back to expiry",
			fi:   instanceInfo{Expiry: t10},
			want: "2026-01-15T10:00:00Z",
		},
		{
			desc: "failure takes precedence over expiry",
			fi:   instanceInfo{SSMResult: &ssmResult{Time: t12}, Expiry: t08},
			want: "2026-01-15T12:00:00Z",
		},
		{
			desc: "no failure and no expiry",
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

func TestInstanceInfo_SSMOutput(t *testing.T) {
	tests := []struct {
		desc string
		fi   instanceInfo
		want string
	}{
		{
			desc: "no SSM failure",
			fi:   instanceInfo{},
			want: "",
		},
		{
			desc: "simple output",
			fi:   instanceInfo{SSMResult: &ssmResult{Output: "install failed"}},
			want: `"install failed"`,
		},
		{
			desc: "newlines escaped",
			fi:   instanceInfo{SSMResult: &ssmResult{Output: "line1\nline2\nline3"}},
			want: `"line1\nline2\nline3"`,
		},
		{
			desc: "carriage returns escaped",
			fi:   instanceInfo{SSMResult: &ssmResult{Output: "line1\r\nline2"}},
			want: `"line1\r\nline2"`,
		},
		{
			desc: "tabs escaped",
			fi:   instanceInfo{SSMResult: &ssmResult{Output: "col1\tcol2"}},
			want: `"col1\tcol2"`,
		},
		{
			desc: "whitespace trimmed",
			fi:   instanceInfo{SSMResult: &ssmResult{Output: "  output  "}},
			want: `"output"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			require.Equal(t, tt.want, tt.fi.ssmOutput())
		})
	}
}

func TestIsSSMFailure(t *testing.T) {
	tests := []struct {
		desc string
		run  *apievents.SSMRun
		want bool
	}{
		{
			desc: "empty status with success code is not a failure",
			run: &apievents.SSMRun{
				Metadata: apievents.Metadata{Code: libevents.SSMRunSuccessCode},
				Status:   "",
			},
			want: false,
		},
		{
			desc: "failure code overrides success status",
			run: &apievents.SSMRun{
				Metadata: apievents.Metadata{Code: libevents.SSMRunFailCode},
				Status:   "Success",
			},
			want: true,
		},
		{
			desc: "non-success status with success code is a failure",
			run: &apievents.SSMRun{
				Metadata: apievents.Metadata{Code: libevents.SSMRunSuccessCode},
				Status:   "Failed",
			},
			want: true,
		},
		{
			desc: "success status with success code is not a failure",
			run: &apievents.SSMRun{
				Metadata: apievents.Metadata{Code: libevents.SSMRunSuccessCode},
				Status:   "Success",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			require.Equal(t, tt.want, isSSMFailure(tt.run))
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
