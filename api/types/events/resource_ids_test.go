/*
Copyright 2026 Gravitational, Inc.

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

package events

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestToEventResourceAccessID(t *testing.T) {
	id := types.ResourceID{ClusterName: "example.teleport.sh", Kind: "node", Name: "server-01"}
	eventID := ResourceID{ClusterName: "example.teleport.sh", Kind: "node", Name: "server-01"}

	tests := []struct {
		name            string
		in              types.ResourceAccessID
		wantConstraints isResourceAccessID_Constraints
	}{
		{
			name:            "no constraints",
			in:              types.ResourceAccessID{Id: id},
			wantConstraints: nil,
		},
		{
			name: "aws console constraints",
			in: types.ResourceAccessID{
				Id: id,
				Constraints: &types.ResourceConstraints{
					Details: &types.ResourceConstraints_AwsConsole{
						AwsConsole: &types.AWSConsoleResourceConstraints{
							RoleArns: []string{"arn:aws:iam::123:role/A", "arn:aws:iam::123:role/B"},
						},
					},
				},
			},
			wantConstraints: &ResourceAccessID_AwsConsole{
				AwsConsole: &AWSConsoleConstraints{
					RoleArnsCount:   2,
					RoleArnsPreview: []string{"arn:aws:iam::123:role/A", "arn:aws:iam::123:role/B"},
				},
			},
		},
		{
			name: "aws console constraints truncated to preview limit",
			in: types.ResourceAccessID{
				Id: id,
				Constraints: &types.ResourceConstraints{
					Details: &types.ResourceConstraints_AwsConsole{
						AwsConsole: &types.AWSConsoleResourceConstraints{
							RoleArns: slices.Repeat([]string{"arn:aws:iam::123456789012:role/Role"}, MaxAuditRoleARNPreview+5),
						},
					},
				},
			},
			wantConstraints: &ResourceAccessID_AwsConsole{
				AwsConsole: &AWSConsoleConstraints{
					RoleArnsCount:   MaxAuditRoleARNPreview + 5,
					RoleArnsPreview: slices.Repeat([]string{"arn:aws:iam::123456789012:role/Role"}, MaxAuditRoleARNPreview),
				},
			},
		},
		{
			name: "ssh constraints",
			in: types.ResourceAccessID{
				Id: id,
				Constraints: &types.ResourceConstraints{
					Details: &types.ResourceConstraints_Ssh{
						Ssh: &types.SSHResourceConstraints{
							Logins: []string{"alice", "root"},
						},
					},
				},
			},
			wantConstraints: &ResourceAccessID_Ssh{
				Ssh: &SSHConstraints{LoginsCount: 2, LoginsPreview: []string{"alice", "root"}},
			},
		},
		{
			name: "unknown constraint type falls back to unknown_constraints",
			in: types.ResourceAccessID{
				Id:          id,
				Constraints: &types.ResourceConstraints{},
			},
			wantConstraints: &ResourceAccessID_UnknownConstraints{UnknownConstraints: &UnknownConstraints{}},
		},
		{
			name: "aws console with nil payload falls back to unknown_constraints",
			in: types.ResourceAccessID{
				Id: id,
				Constraints: &types.ResourceConstraints{
					Details: &types.ResourceConstraints_AwsConsole{AwsConsole: nil},
				},
			},
			wantConstraints: &ResourceAccessID_UnknownConstraints{UnknownConstraints: &UnknownConstraints{}},
		},
		{
			name: "ssh with nil payload falls back to unknown_constraints",
			in: types.ResourceAccessID{
				Id: id,
				Constraints: &types.ResourceConstraints{
					Details: &types.ResourceConstraints_Ssh{Ssh: nil},
				},
			},
			wantConstraints: &ResourceAccessID_UnknownConstraints{UnknownConstraints: &UnknownConstraints{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToEventResourceAccessID(tt.in)
			require.Equal(t, eventID, got.Id)
			require.Equal(t, tt.wantConstraints, got.Constraints)
		})
	}
}

// TestResourceAccessIDJSONRoundTrip is the regression test for the audit-log
// read path. Events are stored as JSON in the file log and decoded via
// FastUnmarshal on read. The protobuf one of generates an interface field
// (isResourceAccessID_Constraints) that encoding/json cannot unmarshal into.
// MarshalJSON/UnmarshalJSON on ResourceAccessID fix this by using an intermediate struct.
func TestResourceAccessIDJSONRoundTrip(t *testing.T) {
	id := ResourceID{ClusterName: "example.teleport.sh", Kind: "node", Name: "server-01"}

	tests := []struct {
		name string
		in   ResourceAccessID
	}{
		{
			name: "no constraints",
			in:   ResourceAccessID{Id: id},
		},
		{
			name: "unknown constraints",
			in: ResourceAccessID{
				Id:          id,
				Constraints: &ResourceAccessID_UnknownConstraints{UnknownConstraints: &UnknownConstraints{}},
			},
		},
		{
			name: "aws console constraints",
			in: ResourceAccessID{
				Id: id,
				Constraints: &ResourceAccessID_AwsConsole{
					AwsConsole: &AWSConsoleConstraints{
						RoleArnsCount:   3,
						RoleArnsPreview: []string{"arn:aws:iam::123:role/A", "arn:aws:iam::123:role/B"},
					},
				},
			},
		},
		{
			name: "ssh constraints",
			in: ResourceAccessID{
				Id: id,
				Constraints: &ResourceAccessID_Ssh{
					Ssh: &SSHConstraints{LoginsCount: 2, LoginsPreview: []string{"alice", "root"}}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(&tt.in)
			require.NoError(t, err)

			var got ResourceAccessID
			require.NoError(t, json.Unmarshal(data, &got))

			require.Equal(t, tt.in.Id, got.Id)
			require.Equal(t, tt.in.Constraints, got.Constraints)
		})
	}
}
