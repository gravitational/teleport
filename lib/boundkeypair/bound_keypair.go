/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package boundkeypair

import (
	"crypto"
	"crypto/subtle"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

const (
	challengeExpiration time.Duration = time.Minute
)

type ChallengeDocument struct {
	*jwt.Claims

	// Nonce is a secure random string, unique to a particular challenge
	Nonce string `json:"nonce"`
}

type ChallengeValidator struct {
	clock clockwork.Clock

	subject     string
	clusterName string
	publicKey   crypto.PublicKey
}

func NewChallengeValidator(
	subject string,
	clusterName string,
	publicKey crypto.PublicKey,
) (*ChallengeValidator, error) {
	return &ChallengeValidator{
		clock: clockwork.NewRealClock(),

		subject:     subject,
		clusterName: clusterName,
		publicKey:   publicKey, // TODO: API design issue, public key will change during rotation. should a new validator be created, or can we design this better?
	}, nil
}

func (v *ChallengeValidator) IssueChallenge() (*ChallengeDocument, error) {
	// Implementation note: these challenges are only ever sent to a single
	// client once, and we expect a valid reply as the next exchange in the
	// join ceremony. There is not an opportunity for reuse, multiple attempts,
	// or for clients to select their own nonce, so we won't bother storing it.
	nonce, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err, "generating nonce")
	}

	return &ChallengeDocument{
		Claims: &jwt.Claims{
			Issuer:    v.clusterName,
			Audience:  jwt.Audience{v.clusterName}, // the cluster is both the issuer and audience
			NotBefore: jwt.NewNumericDate(v.clock.Now().Add(-10 * time.Second)),
			IssuedAt:  jwt.NewNumericDate(v.clock.Now()),
			Expiry:    jwt.NewNumericDate(v.clock.Now().Add(challengeExpiration)),
			Subject:   v.subject,
		},
		Nonce: nonce,
	}, nil
}

func (v *ChallengeValidator) ValidateChallengeResponse(nonce string, compactResponse string) error {
	token, err := jwt.ParseSigned(compactResponse)
	if err != nil {
		return trace.Wrap(err, "parsing signed response")
	}

	var document ChallengeDocument
	if err := token.Claims(v.publicKey, &document); err != nil {
		return trace.Wrap(err)
	}

	// TODO: this doesn't actually validate that the time-based fields are still
	// what we assigned above; a hostile client could set their own values here.
	// This may not be a realistic problem, but we might want to check it
	// anyway.
	const leeway time.Duration = time.Minute
	if err := document.Claims.ValidateWithLeeway(jwt.Expected{
		Issuer:   v.clusterName,
		Subject:  v.subject,
		Audience: jwt.Audience{v.clusterName},
		Time:     v.clock.Now(),
	}, leeway); err != nil {
		return trace.Wrap(err, "validating challenge claims")
	}

	if subtle.ConstantTimeCompare([]byte(nonce), []byte(document.Nonce)) == 0 {
		return trace.AccessDenied("invalid nonce")
	}

	return nil
}
