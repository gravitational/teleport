/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package fakeissuer

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	tokenclaims "github.com/gravitational/teleport/lib/kube/token/claims"
)

// IDP provides a minimal fake OIDC provider for use in tests
type IDP struct {
	log    *slog.Logger
	signer *KubernetesSigner
	server *httptest.Server
}

// NewIDP creates a IDP and starts its HTTP server.
// The caller is responsible for shutting down the server by calling
// IDP.Close().
func NewIDP(log *slog.Logger) (*IDP, error) {
	if log == nil {
		return nil, trace.BadParameter("nil logger")
	}
	signer, err := NewKubernetesSigner(clockwork.NewRealClock())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	f := &IDP{
		signer: signer,
		log:    log,
	}

	providerMux := http.NewServeMux()
	providerMux.HandleFunc(
		"/.well-known/openid-configuration",
		f.handleOpenIDConfig,
	)
	providerMux.HandleFunc(
		"/.well-known/jwks",
		f.handleJWKSEndpoint,
	)

	srv := httptest.NewServer(providerMux)
	f.server = srv
	return f, nil
}

// Close shuts down the server and blocks until all outstanding
// requests on this server have completed.
func (f *IDP) Close() {
	f.server.Close()
}

// IssuerURL returns the URL of the fake IDP.
func (f *IDP) IssuerURL() string {
	return f.server.URL
}

func (f *IDP) handleOpenIDConfig(w http.ResponseWriter, r *http.Request) {
	// mimic `kubectl get --raw /.well-known/openid-configuration` for an EKS cluster
	response := map[string]any{
		"claims_supported": []string{
			"sub",
			"iss",
		},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"issuer":                                f.IssuerURL(),
		"jwks_uri":                              f.IssuerURL() + "/.well-known/jwks",
		"response_types_supported":              []string{"id_token"},
		"scopes_supported":                      []string{"openid"},
		"subject_types_supported":               []string{"public"},
	}
	responseBytes, err := json.Marshal(response)
	if err != nil {
		f.log.ErrorContext(r.Context(), "failed to marshal openid-configuration", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, err = w.Write(responseBytes)
	if err != nil {
		f.log.ErrorContext(r.Context(), "failed to write response", "error", err, "response", string(responseBytes))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (f *IDP) handleJWKSEndpoint(w http.ResponseWriter, r *http.Request) {
	response, err := f.signer.GetMarshaledJWKS()
	if err != nil {
		f.log.ErrorContext(r.Context(), "failed to marshall JWKS", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, err = w.Write([]byte(response))
	if err != nil {
		f.log.ErrorContext(r.Context(), "failed to write JWKS response", "error", err, "response", response)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// IssueToken makes the IDP sign a token for the
// chosen claims and expiry.
func (f *IDP) IssueToken(
	issuer,
	audience,
	sub string,
	issuedAt time.Time,
	expiry time.Time,
	k8s *tokenclaims.KubernetesSubClaim,
) (string, error) {
	claims := tokenclaims.OIDCServiceAccountClaims{
		TokenClaims: oidc.TokenClaims{
			Issuer:     issuer,
			Subject:    sub,
			Audience:   oidc.Audience{audience},
			IssuedAt:   oidc.FromTime(issuedAt),
			NotBefore:  oidc.FromTime(issuedAt),
			Expiration: oidc.FromTime(expiry),
		},
		Kubernetes: k8s,
	}

	return f.signer.signWithClaims(claims)
}

// IssueKubeToken makes the IDP sign a token for a Kubernetes pod.
func (f *IDP) IssueKubeToken(pod, namespace, serviceAccount, clusterName string) (string, error) {
	now := f.signer.clock.Now()
	claims := tokenclaims.OIDCServiceAccountClaims{
		TokenClaims: oidc.TokenClaims{
			Issuer:     f.IssuerURL(),
			Subject:    fmt.Sprintf("system:serviceaccount:%s:%s", namespace, serviceAccount),
			Audience:   oidc.Audience{clusterName},
			IssuedAt:   oidc.FromTime(now.Add(-time.Minute)),
			NotBefore:  oidc.FromTime(now.Add(-time.Minute)),
			Expiration: oidc.FromTime(now.Add(29 * time.Minute)),
		},
		Kubernetes: kubeClaims(pod, namespace, serviceAccount),
	}

	return f.signer.signWithClaims(claims)
}
