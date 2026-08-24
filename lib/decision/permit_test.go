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

package decision

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/constants"
	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
)

const sampleSSHAccessPermitJSON = `{
	"forwardAgent": true,
	"maxConnections": "4",
	"portForwardMode": "SSH_PORT_FORWARD_MODE_LOCAL",
	"clientIdleTimeout": "90s",
	"disconnectExpiredCert": "2026-08-20T12:00:00Z",
	"lockingMode": "strict",
	"bpfEvents": ["command", "network"],
	"lockTargets": [
		{"role": "editor", "login": "bob"},
		{"user": "alice", "mfaDevice": "mfa-device-1"}
	],
	"hostUsersInfo": {
		"groups": ["wheel"],
		"mode": "HOST_USER_MODE_KEEP",
		"uid": "1001",
		"gid": "1001",
		"shell": "/bin/zsh"
	},
	"preconditions": [
		{"kind": "PRECONDITION_KIND_IN_BAND_MFA"}
	]
}`

func sampleSSHAccessPermit() *decisionpb.SSHAccessPermit {
	return decisionpb.SSHAccessPermit_builder{
		ForwardAgent:          true,
		MaxConnections:        4,
		PortForwardMode:       decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_LOCAL,
		ClientIdleTimeout:     durationpb.New(90 * time.Second),
		DisconnectExpiredCert: timestamppb.New(time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)),
		LockingMode:           string(constants.LockingModeStrict),
		BpfEvents:             []string{constants.EnhancedRecordingCommand, constants.EnhancedRecordingNetwork},
		LockTargets: []*decisionpb.LockTarget{
			decisionpb.LockTarget_builder{Role: "editor", Login: "bob"}.Build(),
			decisionpb.LockTarget_builder{User: "alice", MfaDevice: "mfa-device-1"}.Build(),
		},
		HostUsersInfo: decisionpb.HostUsersInfo_builder{
			Groups: []string{"wheel"},
			Mode:   decisionpb.HostUserMode_HOST_USER_MODE_KEEP,
			Uid:    "1001",
			Gid:    "1001",
			Shell:  "/bin/zsh",
		}.Build(),
		Preconditions: []*decisionpb.Precondition{
			decisionpb.Precondition_builder{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA}.Build(),
		},
	}.Build()
}

func TestMarshalSSHAccessPermit(t *testing.T) {
	t.Parallel()

	permit := sampleSSHAccessPermit()

	got, err := MarshalSSHAccessPermit(permit)
	require.NoError(t, err)

	require.JSONEq(t, sampleSSHAccessPermitJSON, got)
}

func TestUnmarshalSSHAccessPermit(t *testing.T) {
	t.Parallel()

	want := sampleSSHAccessPermit()

	got, err := UnmarshalSSHAccessPermit(sampleSSHAccessPermitJSON)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(want, got, protocmp.Transform()), "SSHAccessPermit mismatch (-want +got)")
}

func TestUnmarshalSSHAccessPermit_UnknownFieldAccepted(t *testing.T) {
	t.Parallel()

	want := decisionpb.SSHAccessPermit_builder{
		ForwardAgent: true,
	}.Build()

	got, err := UnmarshalSSHAccessPermit(`{"forward_agent": true, "unknown_field": "something clever"}`)
	require.NoError(t, err, "UnmarshalSSHAccessPermit must accept unknown fields")
	require.Empty(t, cmp.Diff(want, got, protocmp.Transform()), "SSHAccessPermit mismatch (-want +got)")
}

func TestUnmarshalSSHAccessPermit_EmptyInput(t *testing.T) {
	t.Parallel()

	_, err := UnmarshalSSHAccessPermit("")
	require.Error(t, err)
}
