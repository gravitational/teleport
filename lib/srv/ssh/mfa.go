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

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	sshpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/ssh/v1"
)

// ValidatedMFAChallengeVerifier verifies that a validated MFA challenge exists in order to determine if the user has
// completed MFA.
type ValidatedMFAChallengeVerifier interface {
	VerifyValidatedMFAChallenge(ctx context.Context, req *mfav1.VerifyValidatedMFAChallengeRequest, opts ...grpc.CallOption) (*mfav1.VerifyValidatedMFAChallengeResponse, error)
}

// MFAPromptVerifier is a PromptVerifier that marshals and verifies MFA prompts and responses.
type MFAPromptVerifier struct {
	verifier      ValidatedMFAChallengeVerifier
	sourceCluster string
	username      string
	sessionID     []byte
}

var _ PromptVerifier = (*MFAPromptVerifier)(nil)

// NewMFAPromptVerifier creates a new MFAPromptVerifier with the provided parameters.
func NewMFAPromptVerifier(
	verifier ValidatedMFAChallengeVerifier,
	sourceCluster string,
	username string,
	sessionID []byte,
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
	}

	return &MFAPromptVerifier{
		verifier:      verifier,
		sourceCluster: sourceCluster,
		username:      username,
		sessionID:     sessionID,
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

	switch resp := mfaPromptResp.GetResponse().(type) {
	case *sshpb.MFAPromptResponse_Reference:
		challengeName := resp.Reference.GetChallengeName()
		if challengeName == "" {
			return trace.BadParameter("missing ChallengeName in MFAPromptResponseReference")
		}

		req := &mfav1.VerifyValidatedMFAChallengeRequest{
			Name: challengeName,
			Payload: &mfav1.SessionIdentifyingPayload{
				Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
					SshSessionId: pv.sessionID,
				},
			},
			SourceCluster: pv.sourceCluster,
			Username:      pv.username,
		}

		_, err := pv.verifier.VerifyValidatedMFAChallenge(ctx, req)
		return trace.Wrap(err)

	case nil:
		return trace.BadParameter("missing Response in MFAPromptResponse")

	default:
		return trace.BadParameter("unsupported MFAPromptResponse Response type: %T", resp)
	}
}
