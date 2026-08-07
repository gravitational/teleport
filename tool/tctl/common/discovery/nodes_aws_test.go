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

	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
)

const awsStatusSuccess = string(ssmtypes.CommandInvocationStatusSuccess)
const awsStatusFailed = string(ssmtypes.CommandInvocationStatusFailed)

// makeSSMRun creates an *apievents.SSMRun suitable for use in tests.
// Sets the event code based on status: "Success" -> SSMRunSuccessCode, else SSMRunFailCode.
func makeSSMRun(instanceID, accountID, region, status string, exitCode int64, output string, ts time.Time) *apievents.SSMRun {
	code := libevents.SSMRunFailCode
	if strings.EqualFold(status, awsStatusSuccess) {
		code = libevents.SSMRunSuccessCode
	}
	return &apievents.SSMRun{
		Metadata: apievents.Metadata{
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
func makeNode(name, awsInstanceID, accountID, region string, expiry time.Time) *types.ServerV2 {
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

func TestCorrelateSSMEvents(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)

	tests := []struct {
		desc   string
		events []*apievents.SSMRun
		want   map[string]instanceInfo
	}{
		{
			desc: "failures and successes both included",
			events: []*apievents.SSMRun{
				makeSSMRun("i-aaa", "111", "us-east-1", awsStatusFailed, 1, "install failed", now),
				makeSSMRun("i-bbb", "222", "us-west-2", awsStatusSuccess, 0, "", now),
			},
			want: map[string]instanceInfo{
				"i-aaa": {
					AWS:       &awsInfo{InstanceID: "i-aaa", AccountID: "111"},
					Region:    "us-east-1",
					RunResult: &runResult{ExitCode: 1, Output: "install failed", Time: now, IsFailure: true},
				},
				"i-bbb": {
					AWS:       &awsInfo{InstanceID: "i-bbb", AccountID: "222"},
					Region:    "us-west-2",
					RunResult: &runResult{ExitCode: 0, Time: now},
				},
			},
		},
		{
			desc: "empty instance ID is skipped",
			events: []*apievents.SSMRun{
				makeSSMRun("", "", "", awsStatusFailed, 1, "", now),
			},
			want: map[string]instanceInfo{},
		},
		{
			desc: "dedup keeps most recent",
			events: []*apievents.SSMRun{
				makeSSMRun("i-aaa", "111", "us-east-1", awsStatusFailed, 1, "newest error", now),
				makeSSMRun("i-aaa", "111", "us-east-1", awsStatusFailed, 2, "older error", now.Add(-10*time.Minute)),
			},
			want: map[string]instanceInfo{
				"i-aaa": {
					AWS:       &awsInfo{InstanceID: "i-aaa", AccountID: "111"},
					Region:    "us-east-1",
					RunResult: &runResult{ExitCode: 1, Output: "newest error", Time: now, IsFailure: true},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := correlateSSMEvents(tt.events)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCorrelateAWSNodes(t *testing.T) {
	t.Parallel()

	expiry := time.Now().UTC().Truncate(time.Second).Add(time.Hour)

	tests := []struct {
		desc  string
		nodes []types.Server
		want  map[string]instanceInfo
	}{
		{
			desc:  "node with expiry propagates",
			nodes: []types.Server{makeNode("node-1", "i-aaa", "111", "us-east-1", expiry)},
			want: map[string]instanceInfo{
				"i-aaa": {
					IsOnline: true,
					Region:   "us-east-1",
					Expiry:   expiry,
					AWS:      &awsInfo{InstanceID: "i-aaa", AccountID: "111"},
				},
			},
		},
		{
			desc:  "node without AWS instance ID is skipped",
			nodes: []types.Server{makeAzureNode("node-1", "vm-aaa", "sub-1", "rg-1", "eastus", time.Time{})},
			want:  map[string]instanceInfo{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := correlateAWSNodes(tt.nodes)
			require.Equal(t, tt.want, got)
		})
	}
}
