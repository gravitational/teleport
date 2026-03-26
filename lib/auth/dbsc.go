package auth

import (
	"context"
	"encoding/json"

	"github.com/go-jose/go-jose/v3"
	josejwt "github.com/go-jose/go-jose/v3/jwt"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/jwt"
)

type dbscClaims struct {
	josejwt.Claims
}

func (a *Server) verifyDBSCResponse(ctx context.Context, rawJWT string, sessionID string) ([]byte, error) {
	tok, err := josejwt.ParseSigned(rawJWT)
	if err != nil {
		return nil, trace.Wrap(err, "parsing DBSC response JWT")
	}
	if err := validateDBSCProofHeader(tok); err != nil {
		return nil, trace.Wrap(err)
	}

	var claims dbscClaims
	if err := tok.UnsafeClaimsWithoutVerification(&claims); err != nil {
		return nil, trace.Wrap(err, "extracting DBSC claims")
	}

	headerKey := tok.Headers[0].JSONWebKey
	if headerKey == nil {
		return nil, trace.BadParameter("missing jwk header in DBSC response")
	}

	publicKey := headerKey.Public()
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

	var errs []error
	for _, kp := range ca.GetTrustedJWTKeyPairs() {
		publicKey, err := keys.ParsePublicKey(kp.PublicKey)
		if err != nil {
			errs = append(errs, trace.Wrap(err))
			continue
		}

		key, err := jwt.New(&jwt.Config{
			Clock:       a.clock,
			PublicKey:   publicKey,
			ClusterName: ca.GetClusterName(),
		})
		if err != nil {
			errs = append(errs, trace.Wrap(err))
			continue
		}

		err = key.VerifyDBSCChallenge(jwt.DBSCChallengeVerifyParams{
			RawToken:  challenge,
			SessionID: sessionID,
		})
		if err == nil {
			return nil
		}
		errs = append(errs, trace.Wrap(err))
	}

	if len(errs) == 0 {
		return trace.BadParameter("no JWT keys found in CA")
	}

	return trace.Wrap(trace.NewAggregate(errs...), "challenge verification failed")
}

func validateDBSCProofHeader(tok *josejwt.JSONWebToken) error {
	if len(tok.Headers) != 1 {
		return trace.BadParameter("invalid DBSC response JWT header count")
	}

	header := tok.Headers[0]
	if header.Algorithm != string(jose.ES256) && header.Algorithm != string(jose.RS256) {
		return trace.BadParameter("invalid DBSC response alg %q", header.Algorithm)
	}

	typ, ok := header.ExtraHeaders[jose.HeaderKey("typ")]
	if !ok {
		return trace.BadParameter("missing typ header in DBSC response")
	}

	typValue, ok := typ.(string)
	if !ok || typValue != "dbsc+jwt" {
		return trace.BadParameter("invalid typ header %v in DBSC response", typ)
	}

	return nil
}
