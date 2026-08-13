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

package ssh

import (
	"context"
	"slices"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	mfav2pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
	sshpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/ssh/v1"
	"github.com/gravitational/teleport/lib/utils"
)

// ValidatedMFAChallengeVerifier verifies that a validated MFA challenge exists in order to determine if the user has
// completed MFA.
type ValidatedMFAChallengeVerifier interface {
	VerifyValidatedMFAChallenge(ctx context.Context, req *mfav2pb.VerifyValidatedMFAChallengeRequest, opts ...grpc.CallOption) (*mfav2pb.VerifyValidatedMFAChallengeResponse, error)
}

// MFAPromptVerifier is a PromptVerifier that marshals and verifies MFA prompts and responses.
type MFAPromptVerifier struct {
	verifier      ValidatedMFAChallengeVerifier
	sourceCluster string
	username      string
	sessionID     []byte
	perms         *ssh.Permissions
}

var _ PromptVerifier = (*MFAPromptVerifier)(nil)

// NewMFAPromptVerifier creates a new MFAPromptVerifier with the provided parameters.
func NewMFAPromptVerifier(
	verifier ValidatedMFAChallengeVerifier,
	sourceCluster string,
	username string,
	sessionID []byte,
	perms *ssh.Permissions,
) (*MFAPromptVerifier, error) {
	switch {
	case verifier == nil:
		return nil, trace.BadParameter("params Verifier must be set")

	case sourceCluster == "":
		return nil, trace.BadParameter("params SourceCluster must be set")

	case username == "":
		return nil, trace.BadParameter("params Username must be set")

	case len(sessionID) == 0:
		return nil, trace.BadParameter("params SessionID must be set and be non-empty")

	case perms == nil:
		return nil, trace.BadParameter("params Permissions must be set")
	}

	return &MFAPromptVerifier{
		verifier:      verifier,
		sourceCluster: sourceCluster,
		username:      username,
		sessionID:     sessionID,
		perms:         perms,
	}, nil
}

// MFAPromptMessage is the message displayed to users when they are prompted for MFA.
const MFAPromptMessage = "Multi-factor authentication (MFA) is required. Complete the MFA challenge in order to proceed."

// MarshalPrompt returns a JSON-marshaled MFA prompt and an echo flag set to false.
func (pv *MFAPromptVerifier) MarshalPrompt() (string, bool, error) {
	prompt := sshpb.AuthPrompt_builder{
		MfaPrompt: sshpb.MFAPrompt_builder{
			Message: MFAPromptMessage,
		}.Build(),
	}.Build()

	json, err := protojson.Marshal(prompt)
	if err != nil {
		return "", false, trace.Wrap(err)
	}

	return string(json), false, nil
}

// VerifyAnswer verifies the MFA answer by unmarshaling it and checking that the validated MFA challenge exists.
func (pv *MFAPromptVerifier) VerifyAnswer(ctx context.Context, answer string) error {
	mfaPromptResp := &sshpb.MFAPromptResponse{}

	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal([]byte(answer), mfaPromptResp); err != nil {
		return trace.Wrap(err)
	}

	switch resp := mfaPromptResp.WhichResponse(); resp {
	case sshpb.MFAPromptResponse_Reference_case:
		challengeName := mfaPromptResp.GetReference().GetChallengeName()
		if challengeName == "" {
			return trace.BadParameter("missing ChallengeName in MFAPromptResponseReference")
		}

		req := mfav2pb.VerifyValidatedMFAChallengeRequest_builder{
			Name: challengeName,
			Payload: mfav2pb.SessionIdentifyingPayload_builder{
				SshSessionId: pv.sessionID,
			}.Build(),
			SourceCluster: pv.sourceCluster,
			Username:      pv.username,
		}.Build()

		verifyResp, err := pv.verifier.VerifyValidatedMFAChallenge(ctx, req)
		if err != nil {
			return trace.Wrap(err)
		}

		// Capture the MFA device in the decisionpb.SSHAccessPermit.LockTargets for downstream lock enforcement.
		if err := pv.addMFADeviceToPermit(verifyResp.GetMfaDevice()); err != nil {
			return trace.Wrap(err)
		}

		return nil

	case 0:
		return trace.BadParameter("missing Response in MFAPromptResponse")

	default:
		return trace.BadParameter("unsupported MFAPromptResponse Response type: %v", resp)
	}
}

func (pv *MFAPromptVerifier) addMFADeviceToPermit(device *mfav2pb.MFADevice) error {
	if device.GetId() == "" {
		return trace.BadParameter("missing MFA device with non-empty ID (this is a bug)")
	}

	rawPermit, ok := pv.perms.Extensions[utils.ExtIntSSHAccessPermit]
	if !ok {
		return trace.BadParameter("missing SSH access permit (this is a bug)")
	}

	permit := &decisionpb.SSHAccessPermit{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal([]byte(rawPermit), permit); err != nil {
		return trace.Wrap(err)
	}

	// Add the device only if it's not already in the permit.
	if !slices.ContainsFunc(
		permit.GetLockTargets(),
		func(target *decisionpb.LockTarget) bool {
			return target.GetMfaDevice() == device.GetId()
		},
	) {
		permit.SetLockTargets(append(
			permit.GetLockTargets(),
			decisionpb.LockTarget_builder{
				MfaDevice: device.GetId(),
			}.Build(),
		))

		permitJSON, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(permit)
		if err != nil {
			return trace.Wrap(err)
		}

		pv.perms.Extensions[utils.ExtIntSSHAccessPermit] = string(permitJSON)
	}

	return nil
}
