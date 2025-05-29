// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package boundkeypair

import (
	"crypto"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	libjwt "github.com/gravitational/teleport/lib/jwt"
)

// JoinState is a signed JWT stored on joining clients alongside their usual
// certificate bundle, used as an additional layer of verification for
// subsequent join attempts.
type JoinState struct {
	*jwt.Claims

	// BotInstanceID is the bot instance ID associated with this join state when
	// it was generated. A new bot instance may be created during the joining
	// process if the previous instance expired. The
	// `(bot_instance_id, recovery_sequence)` tuple is considered functionally
	// unique to each issued join state for verification purposes, and will
	// remain unchanged until the next successful recovery.
	BotInstanceID string `json:"bot_instance_id"`

	// RecoverySequence is the recovery sequence number. This is incremented
	// each time a recovery is performed, including on first join. This counter
	// is not reset if a new bot instance is generated.
	RecoverySequence uint32 `json:"recovery_sequence"`

	// RecoveryLimit is the maximum number of recovery attempts allowed as of
	// the time this join state was issued. This field is informational, and is
	// expected to be modified server-side to allow additional joins once the
	// limit is reached.
	RecoveryLimit uint32 `json:"recovery_limit"`

	// RecoveryMode is the currently configured recovery mode set in
	// `spec.bound_keypair.recovery.mode`. This field is informational; clients
	// may opt to use this and the recovery limit to, e.g., generate a warning
	// if recovery limits are enforced and the remaining attempts are below some
	// threshold.
	RecoveryMode string `json:"recovery_mode"`
}

// JoinStateParams contains parameters for issuing and verifying join state
// JWTs.
type JoinStateParams struct {
	Clock clockwork.Clock

	ClusterName string
	Token       *types.ProvisionTokenV2
}

// IssueJoinState generates a join state document from the provided token and
// returns a compact serialized, signed JWT. The token must be up-to-date at the
// time of issuance, i.e. the recovery count must have been incremented already.
func IssueJoinState(signer crypto.Signer, params *JoinStateParams) (string, error) {
	spec := params.Token.Spec.BoundKeypair
	if spec == nil {
		return "", trace.BadParameter("spec.bound_keypair: required field is missing")
	}

	if params.Token.Status == nil || params.Token.Status.BoundKeypair == nil {
		return "", trace.BadParameter("status.bound_keypair: required field is missing")
	}

	status := params.Token.Status.BoundKeypair

	if params.Token.Spec.BotName == "" {
		return "", trace.BadParameter("spec.bot_name: required field is empty")
	}

	state := &JoinState{
		Claims: &jwt.Claims{
			// We'll reuse the challengeNotBeforeOffset here; the value is sane
			// enough.
			NotBefore: jwt.NewNumericDate(params.Clock.Now().Add(challengeNotBeforeOffset)),
			IssuedAt:  jwt.NewNumericDate(params.Clock.Now()),
			Issuer:    params.ClusterName,
			Audience:  jwt.Audience{params.ClusterName},
			Subject:   params.Token.Spec.BotName,

			// Note: These documents aren't meant to expire, so no expiration is
			// included. We may opt to trust (or not) a given document during
			// verification based on its `iat` in the future.
		},
		BotInstanceID:    status.BoundBotInstanceID,
		RecoverySequence: status.RecoveryCount,
		RecoveryLimit:    spec.Recovery.Limit,
		RecoveryMode:     spec.Recovery.Mode,
	}

	// Derive the key ID for inclusion in the header.
	kid, err := libjwt.KeyID(signer.Public())
	if err != nil {
		return "", trace.Wrap(err, "generating key ID")
	}

	signingKey, err := libjwt.SigningKeyFromPrivateKey(signer)
	if err != nil {
		return "", trace.Wrap(err)
	}

	opts := (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", kid)
	joseSigner, err := jose.NewSigner(signingKey, opts)
	if err != nil {
		return "", trace.Wrap(err, "creating signer")
	}

	serialized, err := jwt.Signed(joseSigner).Claims(state).CompactSerialize()
	if err != nil {
		return "", trace.Wrap(err)
	}

	return serialized, nil
}

func verifyJoinStateInner(key crypto.PublicKey, parsed *jwt.JSONWebToken, params *JoinStateParams) (*JoinState, error) {
	if params.Token.Status == nil || params.Token.Status.BoundKeypair == nil {
		return nil, trace.BadParameter("invalid token status")
	}

	var document JoinState
	if err := parsed.Claims(key, &document); err != nil {
		return nil, trace.Wrap(err)
	}

	// Note: We don't verify expiry here, only `iat` and `nbf`. These documents
	// are meant to remain "valid" indefinitely and we decide to trust them (or
	// not) at verification time.
	// There are no time-based recovery restrictions yet, but we may opt to add
	// some in the future based on the verified `iat` value of this JWT.
	const leeway time.Duration = time.Minute
	if err := document.Claims.ValidateWithLeeway(jwt.Expected{
		Issuer:   params.ClusterName,
		Audience: jwt.Audience{params.ClusterName},
		Subject:  params.Token.Spec.BotName,
		Time:     params.Clock.Now(),
	}, leeway); err != nil {
		return nil, trace.Wrap(err, "validating join state claims")
	}

	// Ensure the non-informational claims in the join state match what we
	// expect.
	var errors []error
	if document.RecoverySequence != params.Token.Status.BoundKeypair.RecoveryCount {
		errors = append(errors, trace.AccessDenied("recovery counter mismatch"))
	}
	if document.BotInstanceID != params.Token.Status.BoundKeypair.BoundBotInstanceID {
		errors = append(errors, trace.AccessDenied("bot instance mismatch"))
	}

	if len(errors) > 0 {
		return nil, trace.NewAggregate(errors...)
	}

	return &document, nil
}

// VerifyJoinState attempts to verify the given serialized join state JWT
// against the trusted keys in the provided CA and expected join state
// parameters. Note that verification must take place before join state has been
// modified; that is, if a new bot instance is generated or the recovery counter
// is incremented, verification must be done against the original state.
func VerifyJoinState(ca types.CertAuthority, serializedJoinState string, params *JoinStateParams) (*JoinState, error) {
	parsed, err := jwt.ParseSigned(serializedJoinState)
	if err != nil {
		return nil, trace.Wrap(err, "parsing serialized join state")
	}

	if len(parsed.Headers) == 0 {
		return nil, trace.BadParameter("invalid JWT header")
	}

	expectedKeyID := parsed.Headers[0].KeyID
	if expectedKeyID == "" {
		return nil, trace.BadParameter("required key ID is missing from JWT header")
	}

	// Attempt to find the key that signed this JWT.
	for _, k := range ca.GetTrustedJWTKeyPairs() {
		pubKey, err := keys.ParsePublicKey(k.PublicKey)
		if err != nil {
			return nil, trace.Wrap(err, "parsing public key")
		}

		kid, err := libjwt.KeyID(pubKey)
		if err != nil {
			return nil, trace.Wrap(err, "deriving key ID")
		}

		if kid == expectedKeyID {
			joinState, err := verifyJoinStateInner(pubKey, parsed, params)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return joinState, err
		}
	}

	// No matching keys were found, bail.
	return nil, trace.AccessDenied("join state could not be verified")
}
