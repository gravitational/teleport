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

package desktop

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
	"github.com/gravitational/teleport/lib/srv/mfa"
)

// challengeVerifier verifies that a validated MFA challenge exists.
type challengeVerifier interface {
	VerifyValidatedMFAChallenge(
		ctx context.Context,
		req *mfav2.VerifyValidatedMFAChallengeRequest,
		opts ...grpc.CallOption,
	) (*mfav2.VerifyValidatedMFAChallengeResponse, error)
}

// MFAPromptVerifier verifies MFA prompts and responses for desktop in-band MFA.
type MFAPromptVerifier struct {
	mfa.Verifier
}

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

// NewAuthPrompt creates a TDPB AuthPrompt containing an empty MFAPrompt to send to the client.
func NewAuthPrompt() *tdpbv1.AuthPrompt {
	return &tdpbv1.AuthPrompt{
		Prompt: &tdpbv1.AuthPrompt_MfaPrompt{
			MfaPrompt: &tdpbv1.MFAPrompt{},
		},
	}
}

// VerifyResponse verifies the MFA response by extracting the challenge name and checking that the validated MFA
// challenge exists.
func (pv *MFAPromptVerifier) VerifyResponse(ctx context.Context, resp *tdpbv1.MFAPromptResponse) error {
	switch r := resp.GetResponse().(type) {
	case *tdpbv1.MFAPromptResponse_Reference:
		return pv.Verify(
			ctx,
			r.Reference.GetChallengeName(),
			func() *mfav2.SessionIdentifyingPayload {
				return mfav2.SessionIdentifyingPayload_builder{
					TlsSessionId: pv.SessionID(),
				}.Build()
			},
		)

	default:
		return trace.BadParameter("missing or unknown MFAPromptResponse type: %T", r)
	}
}
