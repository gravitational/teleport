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
	"context"
	"crypto"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cryptosuites"
	libjwt "github.com/gravitational/teleport/lib/jwt"
)

func newTestKeypair(t *testing.T) crypto.Signer {
	t.Helper()

	key, err := cryptosuites.GenerateKey(context.Background(), func(ctx context.Context) (types.SignatureAlgorithmSuite, error) {
		return types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1, nil
	}, cryptosuites.BoundKeypairJoining)
	require.NoError(t, err)

	return key
}

func TestChallengeValidator_IssueChallenge(t *testing.T) {
	t.Parallel()

	key := newTestKeypair(t)

	clock := clockwork.NewFakeClock()
	now := clock.Now()

	const clusterName = "example.teleport.sh"
	validator, err := NewChallengeValidator("subject", clusterName, key.Public())
	require.NoError(t, err)

	validator.clock = clock

	challenge, err := validator.IssueChallenge()
	require.NoError(t, err)

	require.NotEmpty(t, challenge.Nonce)
	require.Equal(t, clusterName, challenge.Issuer)
	require.Equal(t, jwt.Audience{clusterName}, challenge.Audience)
	require.Equal(t, "subject", challenge.Subject)
	require.Equal(t, jwt.NewNumericDate(now), challenge.IssuedAt)
	require.Equal(t, jwt.NewNumericDate(now.Add(challengeNotBeforeOffset)), challenge.NotBefore)
	require.Equal(t, jwt.NewNumericDate(now.Add(challengeExpiration)), challenge.Expiry)

	newChallenge, err := validator.IssueChallenge()
	require.NoError(t, err)

	require.NotEqual(t, challenge.Nonce, newChallenge.Nonce, "nonces must be random")
}

func signChallenge(t *testing.T, challenge string, signer crypto.Signer) string {
	t.Helper()

	alg, err := libjwt.AlgorithmForPublicKey(signer.Public())
	require.NoError(t, err)

	opts := (&jose.SignerOptions{}).WithType("JWT")
	key := jose.SigningKey{
		Algorithm: alg,
		Key:       signer,
	}

	joseSigner, err := jose.NewSigner(key, opts)
	require.NoError(t, err)

	jws, err := joseSigner.Sign([]byte(challenge))
	require.NoError(t, err)

	serialized, err := jws.CompactSerialize()
	require.NoError(t, err)

	return serialized
}

func jsonClone(t *testing.T, c *ChallengeDocument) *ChallengeDocument {
	t.Helper()

	bytes, err := json.Marshal(c)
	require.NoError(t, err)

	var cloned ChallengeDocument
	require.NoError(t, json.Unmarshal(bytes, &cloned))

	return &cloned
}

func TestChallengeValidator_ValidateChallengeResponse(t *testing.T) {
	t.Parallel()

	correctKey := newTestKeypair(t)
	incorrectKey := newTestKeypair(t)

	const clusterName = "example.teleport.sh"

	tests := []struct {
		name         string
		key          crypto.Signer
		assert       require.ErrorAssertionFunc
		clockFn      func(clock *clockwork.FakeClock)
		manipulateFn func(doc *ChallengeDocument, now time.Time)
	}{
		{
			name:   "success",
			key:    correctKey,
			assert: require.NoError,
		},
		{
			name:   "wrong key",
			key:    incorrectKey,
			assert: require.Error,
		},
		{
			name: "waited too long",
			key:  correctKey,
			clockFn: func(clock *clockwork.FakeClock) {
				clock.Advance(challengeExpiration * 10)
			},
			assert: func(tt require.TestingT, err error, i ...interface{}) {
				require.Error(tt, err)
				require.ErrorContains(tt, err, "token is expired")
			},
		},
		{
			name: "too early",
			key:  correctKey,
			clockFn: func(clock *clockwork.FakeClock) {
				clock.Advance(challengeNotBeforeOffset * 10)
			},
			assert: func(tt require.TestingT, err error, i ...interface{}) {
				require.Error(tt, err)
				require.ErrorContains(tt, err, "token not valid yet")
			},
		},
		{
			name: "tampered with iat",
			key:  correctKey,
			manipulateFn: func(doc *ChallengeDocument, now time.Time) {
				doc.IssuedAt = jwt.NewNumericDate(now.Add(time.Minute))
			},
			assert: func(tt require.TestingT, err error, i ...interface{}) {
				require.Error(tt, err)
				require.ErrorContains(tt, err, "invalid challenge document")
			},
		},
		{
			name: "tampered with exp",
			key:  correctKey,
			manipulateFn: func(doc *ChallengeDocument, now time.Time) {
				doc.Expiry = jwt.NewNumericDate(now.Add(time.Hour))
			},
			assert: func(tt require.TestingT, err error, i ...interface{}) {
				require.Error(tt, err)
				require.ErrorContains(tt, err, "invalid challenge document")
			},
		},
		{
			name: "tampered with nbf",
			key:  correctKey,
			manipulateFn: func(doc *ChallengeDocument, now time.Time) {
				doc.NotBefore = jwt.NewNumericDate(now.Add(time.Minute))
			},
			assert: func(tt require.TestingT, err error, i ...interface{}) {
				require.Error(tt, err)
				require.ErrorContains(tt, err, "invalid challenge document")
			},
		},
		{
			name: "tampered with nonce",
			key:  correctKey,
			manipulateFn: func(doc *ChallengeDocument, now time.Time) {
				doc.Nonce = "abcd"
			},
			assert: func(tt require.TestingT, err error, i ...interface{}) {
				require.Error(tt, err)
				require.ErrorContains(tt, err, "invalid nonce")
			},
		},
		{
			name: "tampered with subject",
			key:  correctKey,
			manipulateFn: func(doc *ChallengeDocument, now time.Time) {
				doc.Subject = "abcd"
			},
			assert: func(tt require.TestingT, err error, i ...interface{}) {
				require.Error(tt, err)
				require.ErrorContains(tt, err, "invalid subject claim")
			},
		},
		{
			name: "tampered with issuer",
			key:  correctKey,
			manipulateFn: func(doc *ChallengeDocument, now time.Time) {
				doc.Issuer = "abcd"
			},
			assert: func(tt require.TestingT, err error, i ...interface{}) {
				require.Error(tt, err)
				require.ErrorContains(tt, err, "invalid issuer claim")
			},
		},
		{
			name: "tampered with audience",
			key:  correctKey,
			manipulateFn: func(doc *ChallengeDocument, now time.Time) {
				doc.Audience = jwt.Audience{"abcd"}
			},
			assert: func(tt require.TestingT, err error, i ...interface{}) {
				require.Error(tt, err)
				require.ErrorContains(tt, err, "invalid audience claim")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := clockwork.NewFakeClock()

			validator, err := NewChallengeValidator("subject", clusterName, correctKey.Public())
			require.NoError(t, err)

			validator.clock = clock

			challenge, err := validator.IssueChallenge()
			require.NoError(t, err)

			cloned := jsonClone(t, challenge)
			if tt.manipulateFn != nil {
				tt.manipulateFn(cloned, clock.Now())
			}

			challengeString, err := json.Marshal(cloned)
			require.NoError(t, err)

			signed := signChallenge(t, string(challengeString), tt.key)

			if tt.clockFn != nil {
				tt.clockFn(clock)
			}

			err = validator.ValidateChallengeResponse(challenge, signed)
			tt.assert(t, err)
		})
	}
}
