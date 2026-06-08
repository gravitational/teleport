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

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"

	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
	sshpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/ssh/v1"
	"github.com/gravitational/teleport/lib/srv/mfa"
)

// challengeVerifier verifies that a validated MFA challenge exists.
type challengeVerifier interface {
	VerifyValidatedMFAChallenge(ctx context.Context, req *mfav2.VerifyValidatedMFAChallengeRequest, opts ...grpc.CallOption) (*mfav2.VerifyValidatedMFAChallengeResponse, error)
}

// MFAPromptVerifier is a PromptVerifier that marshals and verifies MFA prompts and responses.
type MFAPromptVerifier struct {
	mfa.Verifier
}

var _ PromptVerifier = (*MFAPromptVerifier)(nil)

// NewMFAPromptVerifier creates a new MFAPromptVerifier with the provided parameters.
func NewMFAPromptVerifier(
	challengeVerifier challengeVerifier,
	sourceCluster string,
	username string,
	sessionID []byte,
) (*MFAPromptVerifier, error) {
	mfaVerifier, err := mfa.NewVerifier(challengeVerifier, sourceCluster, username, sessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &MFAPromptVerifier{
		Verifier: *mfaVerifier,
	}, nil
}

// MFAPromptMessage is the message displayed to users when they are prompted for MFA.
const MFAPromptMessage = "Multi-factor authentication (MFA) is required. Complete the MFA challenge in order to proceed."

// MarshalPrompt returns a JSON-marshaled MFA prompt and an echo flag set to false.
func (pv *MFAPromptVerifier) MarshalPrompt() (string, bool, error) {
	prompt := &sshpb.AuthPrompt{
		Prompt: &sshpb.AuthPrompt_MfaPrompt{
			MfaPrompt: &sshpb.MFAPrompt{
				Message: MFAPromptMessage,
			},
		},
	}

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

	switch r := mfaPromptResp.GetResponse().(type) {
	case *sshpb.MFAPromptResponse_Reference:
		return pv.Verify(ctx, r.Reference.GetChallengeName(), func() *mfav2.SessionIdentifyingPayload {
			return mfav2.SessionIdentifyingPayload_builder{
				SshSessionId: pv.SessionID(),
			}.Build()
		})

	default:
		return trace.BadParameter("missing or unknown MFAPromptResponse type: %T", r)
	}
}
