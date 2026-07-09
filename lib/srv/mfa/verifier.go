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

package mfa

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
)

// verifyTimeout is the maximum time allowed for an MFA challenge verification RPC. This matches the 3-minute MFA
// enforcement timeout defined in RFD 0234/0314.
const verifyTimeout = 3 * time.Minute

// challengeVerifier verifies that a validated MFA challenge exists.
type challengeVerifier interface {
	VerifyValidatedMFAChallenge(ctx context.Context, req *mfav2.VerifyValidatedMFAChallengeRequest, opts ...grpc.CallOption) (*mfav2.VerifyValidatedMFAChallengeResponse, error)
}

// Verifier verifies MFA challenge responses for in-band MFA. It enforces timeout and validation, delegating
// protocol-specific payload construction to the caller.
type Verifier struct {
	challengeVerifier challengeVerifier
	sourceCluster     string
	username          string
	sessionID         []byte
	timeout           time.Duration
}

// NewVerifier creates a new Verifier with the provided parameters.
func NewVerifier(
	challengeVerifier challengeVerifier,
	sourceCluster string,
	username string,
	sessionID []byte,
) (*Verifier, error) {
	switch {
	case challengeVerifier == nil:
		return nil, trace.BadParameter("params ChallengeVerifier must be set")

	case sourceCluster == "":
		return nil, trace.BadParameter("params SourceCluster must be set")

	case username == "":
		return nil, trace.BadParameter("params Username must be set")

	case len(sessionID) == 0:
		return nil, trace.BadParameter("params SessionID must be set and be non-empty")
	}

	return &Verifier{
		challengeVerifier: challengeVerifier,
		sourceCluster:     sourceCluster,
		username:          username,
		sessionID:         sessionID,
		timeout:           verifyTimeout,
	}, nil
}

// Verify validates the MFA challenge by enforcing timeout and validation, then calling the MFA service. The
// buildPayload function is responsible for constructing the protocol-specific SessionIdentifyingPayload.
func (v *Verifier) Verify(ctx context.Context, challengeName string, buildPayload func() *mfav2.SessionIdentifyingPayload) error {
	if challengeName == "" {
		return trace.BadParameter("missing ChallengeName in MFAPromptResponseReference")
	}

	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	req := mfav2.VerifyValidatedMFAChallengeRequest_builder{
		Name:          challengeName,
		Payload:       buildPayload(),
		SourceCluster: v.sourceCluster,
		Username:      v.username,
	}.Build()

	if _, err := v.challengeVerifier.VerifyValidatedMFAChallenge(ctx, req); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// SessionID returns the session identifying payload for this verifier.
func (v *Verifier) SessionID() []byte {
	return v.sessionID
}
