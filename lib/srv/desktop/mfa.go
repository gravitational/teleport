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
	"crypto/tls"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
	srvmfa "github.com/gravitational/teleport/lib/srv/mfa"
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
	srvmfa.Verifier
}

// NewMFAPromptVerifier creates a new MFAPromptVerifier with the provided parameters.
func NewMFAPromptVerifier(
	challengeVerifier challengeVerifier,
	sourceCluster string,
	username string,
	sessionID []byte,
) (*MFAPromptVerifier, error) {
	mfaVerifier, err := srvmfa.NewVerifier(challengeVerifier, sourceCluster, username, sessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &MFAPromptVerifier{
		Verifier: *mfaVerifier,
	}, nil
}

// NewAuthPrompt creates a TDPB AuthPrompt containing an empty MFAPrompt to send to the client.
func NewAuthPrompt() *tdpbv1.AuthPrompt {
	return tdpbv1.AuthPrompt_builder{
		MfaPrompt: &tdpbv1.MFAPrompt{},
	}.Build()
}

// VerifyResponse verifies the MFA response by extracting the challenge name and checking that the validated MFA
// challenge exists.
func (pv *MFAPromptVerifier) VerifyResponse(ctx context.Context, resp *tdpbv1.MFAPromptResponse) error {
	switch r := resp.WhichResponse(); r {
	case tdpbv1.MFAPromptResponse_Reference_case:
		return pv.Verify(
			ctx,
			resp.GetReference().GetChallengeName(),
			func() *mfav2.SessionIdentifyingPayload {
				return mfav2.SessionIdentifyingPayload_builder{
					TlsSessionId: pv.SessionID(),
				}.Build()
			},
		)

	default:
		return trace.BadParameter("missing or unknown MFAPromptResponse type: %v", r)
	}
}

// errInBandMFARequired is returned when in-band MFA is required but the client does not support it.
var errInBandMFARequired = trace.AccessDenied(
	"This connection requires in-band MFA, but your desktop client does not support it. " +
		"Please update your Teleport desktop client to the latest version to connect.",
)

// HandleInBandMFA performs the in-band MFA exchange with the client. It computes the SIP from the TLS connection,
// sends an MFA prompt, reads the client's response, and verifies the validated MFA challenge.
func HandleInBandMFA(
	ctx context.Context,
	tlsConn *tls.Conn,
	conn tdp.MessageReadWriteCloser,
	cv challengeVerifier,
	sourceCluster string,
	username string,
) error {
	sip, err := mfa.DeriveSIP(tlsConn)
	if err != nil {
		return trace.Wrap(err)
	}

	verifier, err := NewMFAPromptVerifier(cv, sourceCluster, username, sip)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := conn.WriteMessage((*tdpb.AuthPrompt)(NewAuthPrompt())); err != nil {
		return trace.Wrap(err)
	}

	// Enforce the 3-minute MFA timeout. The underlying tls.Conn is shared with the tdp.Conn, so the deadline applies to
	// the ReadMessage call below.
	if err := tlsConn.SetReadDeadline(time.Now().Add(3 * time.Minute)); err != nil {
		return trace.Wrap(err)
	}
	defer tlsConn.SetReadDeadline(time.Time{})

	msg, err := conn.ReadMessage()
	if err != nil {
		return trace.Wrap(err)
	}

	resp, ok := msg.(*tdpb.MFAPromptResponse)
	if !ok {
		return trace.BadParameter("expected MFAPromptResponse, got %T", msg)
	}

	if err := verifier.VerifyResponse(ctx, (*tdpbv1.MFAPromptResponse)(resp)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
