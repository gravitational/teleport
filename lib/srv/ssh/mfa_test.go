/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package ssh_test

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
	sshpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/ssh/v1"
	srvssh "github.com/gravitational/teleport/lib/srv/ssh"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	sessionID        = "test-session-id"
	challengeName    = "test-challenge-name"
	sourceCluster    = "test-cluster"
	teleportUsername = "alice"
	testDeviceID     = "test-device-id"
)

func TestNewMFAPromptVerifier_InvalidParams(t *testing.T) {
	t.Parallel()

	type params struct {
		verifier      srvssh.ValidatedMFAChallengeVerifier
		sourceCluster string
		username      string
		sessionID     []byte
		perms         *ssh.Permissions
	}

	baseParams := params{
		verifier:      &mockValidatedMFAChallengeVerifier{},
		sourceCluster: sourceCluster,
		username:      teleportUsername,
		sessionID:     []byte(sessionID),
		perms:         newPerms(t),
	}

	for _, testCase := range []struct {
		name    string
		params  params
		wantErr error
	}{
		{
			name: "nil verifier",
			params: func() params {
				p := baseParams
				p.verifier = nil
				return p
			}(),
			wantErr: trace.BadParameter("params Verifier must be set"),
		},
		{
			name: "empty sourceCluster",
			params: func() params {
				p := baseParams
				p.sourceCluster = ""
				return p
			}(),
			wantErr: trace.BadParameter("params SourceCluster must be set"),
		},
		{
			name: "empty username",
			params: func() params {
				p := baseParams
				p.username = ""
				return p
			}(),
			wantErr: trace.BadParameter("params Username must be set"),
		},
		{
			name: "nil sessionID",
			params: func() params {
				p := baseParams
				p.sessionID = nil
				return p
			}(),
			wantErr: trace.BadParameter("params SessionID must be set and be non-empty"),
		},
		{
			name: "empty sessionID",
			params: func() params {
				p := baseParams
				p.sessionID = []byte("")
				return p
			}(),
			wantErr: trace.BadParameter("params SessionID must be set and be non-empty"),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := srvssh.NewMFAPromptVerifier(
				testCase.params.verifier,
				testCase.params.sourceCluster,
				testCase.params.username,
				testCase.params.sessionID,
				testCase.params.perms,
			)
			require.ErrorIs(t, err, testCase.wantErr)
		})
	}
}

func TestMFAPromptVerifier_MarshalPrompt(t *testing.T) {
	t.Parallel()

	verifier, err := srvssh.NewMFAPromptVerifier(
		&mockValidatedMFAChallengeVerifier{expectedChallengeName: challengeName},
		sourceCluster,
		teleportUsername,
		[]byte(sessionID),
		newPerms(t),
	)
	require.NoError(t, err)

	prompt, echo, err := verifier.MarshalPrompt()
	require.NoError(t, err)
	require.False(t, echo)
	require.Contains(t, prompt, srvssh.MFAPromptMessage)
}

func TestMFAPromptVerifier_VerifyAnswer(t *testing.T) {
	t.Parallel()

	validRefResp := sshpb.MFAPromptResponse_builder{
		Reference: sshpb.MFAPromptResponseReference_builder{
			ChallengeName: challengeName,
		}.Build(),
	}.Build()
	validRefJSON, err := protojson.Marshal(validRefResp)
	require.NoError(t, err)

	emptyRespJSON, err := protojson.Marshal(sshpb.MFAPromptResponse_builder{}.Build())
	require.NoError(t, err)

	emptyChallengeJSON, err := protojson.Marshal(sshpb.MFAPromptResponse_builder{
		Reference: sshpb.MFAPromptResponseReference_builder{
			ChallengeName: "",
		}.Build(),
	}.Build())
	require.NoError(t, err)

	for _, tc := range []struct {
		name          string
		perms         *ssh.Permissions
		mockMFADevice *mfav2.MFADevice
		answer        string
		assert        func(t *testing.T, err error, perms *ssh.Permissions)
	}{
		{
			name:   "success",
			answer: string(validRefJSON),
			assert: func(t *testing.T, err error, perms *ssh.Permissions) {
				require.NoError(t, err)
				permit := &decisionpb.SSHAccessPermit{}
				require.NoError(t, (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal([]byte(perms.Extensions[utils.ExtIntSSHAccessPermit]), permit))
				require.Len(t, permit.GetLockTargets(), 1)
				require.Equal(t, testDeviceID, permit.GetLockTargets()[0].GetMfaDevice())
			},
		},
		{
			name:   "invalid JSON",
			answer: "not-json",
			assert: func(t *testing.T, err error, perms *ssh.Permissions) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "invalid value not-json")
			},
		},
		{
			name:   "missing Response",
			answer: string(emptyRespJSON),
			assert: func(t *testing.T, err error, perms *ssh.Permissions) {
				require.ErrorIs(t, err, trace.BadParameter("missing Response in MFAPromptResponse"))
			},
		},
		{
			name:   "empty ChallengeName",
			answer: string(emptyChallengeJSON),
			assert: func(t *testing.T, err error, perms *ssh.Permissions) {
				require.ErrorIs(t, err, trace.BadParameter("missing ChallengeName in MFAPromptResponseReference"))
			},
		},
		{
			name:          "empty MFA device",
			mockMFADevice: mfav2.MFADevice_builder{Id: ""}.Build(),
			answer:        string(validRefJSON),
			assert: func(t *testing.T, err error, perms *ssh.Permissions) {
				require.ErrorIs(t, err, trace.BadParameter("missing MFA device with non-empty ID (this is a bug)"))
			},
		},
		{
			name:   "missing SSHAccessPermit",
			perms:  &ssh.Permissions{},
			answer: string(validRefJSON),
			assert: func(t *testing.T, err error, perms *ssh.Permissions) {
				require.ErrorIs(t, err, trace.BadParameter("missing SSH access permit (this is a bug)"))
			},
		},
		{
			name: "invalid SSHAccessPermit",
			perms: &ssh.Permissions{
				Extensions: map[string]string{
					utils.ExtIntSSHAccessPermit: "not-json",
				},
			},
			answer: string(validRefJSON),
			assert: func(t *testing.T, err error, perms *ssh.Permissions) {
				require.Error(t, err)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			perms := tc.perms
			if perms == nil {
				perms = newPerms(t)
			}

			verifier, err := srvssh.NewMFAPromptVerifier(
				&mockValidatedMFAChallengeVerifier{expectedChallengeName: challengeName, mfaDevice: tc.mockMFADevice},
				sourceCluster,
				teleportUsername,
				[]byte(sessionID),
				perms,
			)
			require.NoError(t, err)

			err = verifier.VerifyAnswer(t.Context(), tc.answer)
			tc.assert(t, err, perms)
		})
	}
}

func newPerms(t *testing.T, mfaDevices ...string) *ssh.Permissions {
	t.Helper()

	lockTargets := make([]*decisionpb.LockTarget, 0, len(mfaDevices))
	for _, device := range mfaDevices {
		lockTargets = append(lockTargets, decisionpb.LockTarget_builder{MfaDevice: device}.Build())
	}

	permit := decisionpb.SSHAccessPermit_builder{
		LockTargets: lockTargets,
	}.Build()
	permitJSON, err := protojson.Marshal(permit)
	require.NoError(t, err)

	return &ssh.Permissions{
		Extensions: map[string]string{
			utils.ExtIntSSHAccessPermit: string(permitJSON),
		},
	}
}

type mockValidatedMFAChallengeVerifier struct {
	expectedChallengeName string
	mfaDevice             *mfav2.MFADevice
	err                   error
}

func (m *mockValidatedMFAChallengeVerifier) VerifyValidatedMFAChallenge(
	_ context.Context,
	req *mfav2.VerifyValidatedMFAChallengeRequest,
	_ ...grpc.CallOption,
) (*mfav2.VerifyValidatedMFAChallengeResponse, error) {
	if m.err != nil {
		return nil, m.err
	}

	if m.expectedChallengeName != "" && req.GetName() != m.expectedChallengeName {
		return nil, trace.Errorf(
			"unexpected challenge name: got %q, want %q",
			req.GetName(),
			m.expectedChallengeName,
		)
	}

	mfaDevice := mfav2.MFADevice_builder{
		Id: testDeviceID,
	}.Build()
	if m.mfaDevice != nil {
		mfaDevice = m.mfaDevice
	}

	return mfav2.VerifyValidatedMFAChallengeResponse_builder{
		MfaDevice: mfaDevice,
	}.Build(), nil
}
