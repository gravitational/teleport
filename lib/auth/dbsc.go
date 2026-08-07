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

package auth

import (
	"context"
	"encoding/json"

	"github.com/go-jose/go-jose/v3"
	josejwt "github.com/go-jose/go-jose/v3/jwt"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
)

type dbscProofClaims struct {
	josejwt.Claims
	Key jose.JSONWebKey `json:"key"`
}

// signDBSCChallenge creates a signed challenge.
func (a *Server) signDBSCChallenge(ctx context.Context, sessionID string) (string, error) {
	clusterName, err := a.GetClusterName(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	ca, err := a.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.JWTSigner,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return "", trace.Wrap(err)
	}

	signer, err := a.GetKeyStore().GetJWTSigner(ctx, ca)
	if err != nil {
		return "", trace.Wrap(err)
	}

	privateKey, err := services.GetJWTSigner(signer, ca.GetClusterName(), a.clock)
	if err != nil {
		return "", trace.Wrap(err)
	}

	challenge, err := privateKey.SignDBSCChallenge(sessionID)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return challenge, nil
}

func (a *Server) verifyDBSCResponse(ctx context.Context, rawJWT string, sessionID string) ([]byte, error) {
	tok, err := josejwt.ParseSigned(rawJWT)
	if err != nil {
		return nil, trace.Wrap(err, "parsing DBSC response JWT")
	}

	if err := jwt.ValidateDBSCProofHeader(tok); err != nil {
		return nil, trace.Wrap(err)
	}

	var claims dbscProofClaims
	if err := tok.UnsafeClaimsWithoutVerification(&claims); err != nil {
		return nil, trace.Wrap(err, "extracting DBSC claims")
	}

	publicKey, err := dbscProofPublicKey(tok.Headers[0].JSONWebKey, &claims.Key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := tok.Claims(&publicKey, &claims); err != nil {
		return nil, trace.Wrap(err, "verifying DBSC response signature")
	}

	if claims.ID == "" {
		return nil, trace.BadParameter("missing jti claim (challenge) in DBSC response")
	}

	if err := a.verifyDBSCChallenge(ctx, claims.ID, sessionID); err != nil {
		return nil, trace.Wrap(err, "verifying DBSC challenge")
	}

	publicKeyJSON, err := json.Marshal(publicKey)
	if err != nil {
		return nil, trace.Wrap(err, "serializing public key")
	}

	return publicKeyJSON, nil
}

func dbscProofPublicKey(headerKey, claimsKey *jose.JSONWebKey) (jose.JSONWebKey, error) {
	switch {
	case headerKey != nil && claimsKey != nil && claimsKey.Key != nil:
		headerPublicKey := headerKey.Public()
		claimsPublicKey := claimsKey.Public()
		headerPublicKeyJSON, err := json.Marshal(headerPublicKey)
		if err != nil {
			return jose.JSONWebKey{}, trace.Wrap(err, "serializing DBSC response header public key")
		}
		claimsPublicKeyJSON, err := json.Marshal(claimsPublicKey)
		if err != nil {
			return jose.JSONWebKey{}, trace.Wrap(err, "serializing DBSC response claim public key")
		}
		if string(headerPublicKeyJSON) != string(claimsPublicKeyJSON) {
			return jose.JSONWebKey{}, trace.BadParameter("DBSC response header and claim public keys do not match")
		}
		return headerPublicKey, nil
	case headerKey != nil:
		return headerKey.Public(), nil
	case claimsKey != nil && claimsKey.Key != nil:
		return claimsKey.Public(), nil
	default:
		return jose.JSONWebKey{}, trace.BadParameter("missing public key in DBSC response")
	}
}

func (a *Server) verifyDBSCChallenge(ctx context.Context, challenge string, sessionID string) error {
	clusterName, err := a.GetClusterName(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	ca, err := a.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.JWTSigner,
		DomainName: clusterName.GetClusterName(),
	}, false)
	if err != nil {
		return trace.Wrap(err, "getting JWT CA")
	}

	return jwt.VerifyDBSCChallengeWithCA(jwt.VerifyDBSCChallengeParams{
		Challenge:   challenge,
		SessionID:   sessionID,
		ClusterName: ca.GetClusterName(),
		Clock:       a.clock,
		KeyPairs:    ca.GetTrustedJWTKeyPairs(),
	})
}
