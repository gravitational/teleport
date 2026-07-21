// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package mfa

import (
	"crypto/x509"
	"encoding/pem"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/jwt"
)

// VerifiedToken holds the claims extracted from a verified token.
type VerifiedToken struct {
	Username string
}

// Verifier validates tokens signed by InBandCA.
type Verifier struct {
	issuer      string
	clusterName string
	key         *jwt.Key
}

// NewVerifier creates a verifier from an InBandCA CertAuthority. Extracts the active public key and creates a jwt.Key
// for verification.
func NewVerifier(ca types.CertAuthority, clock clockwork.Clock) (*Verifier, error) {
	pairs := ca.GetTrustedJWTKeyPairs()
	if len(pairs) == 0 {
		return nil, trace.BadParameter("InBandCA has no JWT key pairs")
	}

	block, _ := pem.Decode(pairs[0].PublicKey)
	if block == nil {
		return nil, trace.BadParameter("failed to decode InBandCA public key PEM")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key, err := jwt.New(
		&jwt.Config{
			Clock:       clock,
			ClusterName: ca.GetClusterName(),
			PublicKey:   pub,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Verifier{
		key:         key,
		issuer:      ca.GetClusterName(),
		clusterName: ca.GetClusterName(),
	}, nil
}

// Verify validates a token. Checks signature, issuer, subject, and expiry.
func (v *Verifier) Verify(token string, expectedUser string) (*VerifiedToken, error) {
	claims, err := v.key.Verify(
		jwt.VerifyParams{
			Username: expectedUser,
			RawToken: token,
			URI:      v.clusterName,
			Issuer:   v.issuer,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &VerifiedToken{
		Username: claims.Username,
	}, nil
}

// TODO(cthach): Add SetKeys with atomic.Pointer for key rotation.
// TODO(cthach): Support multiple issuers for trusted cluster tokens.
