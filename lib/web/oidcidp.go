/*
Copyright 2023 Gravitational, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package web

import (
	"context"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/utils/oidc"
)

const (
	// OIDCJWKWURI is the relative path where the OIDC IdP JWKS is located
	OIDCJWKWURI = "/.well-known/jwks-oidc"
)

// openidConfiguration returns the openid-configuration for setting up the AWS OIDC Integration
func (h *Handler) openidConfiguration(_ http.ResponseWriter, _ *http.Request, _ httprouter.Params) (interface{}, error) {
	issuer, err := oidc.IssuerFromPublicAddress(h.cfg.PublicProxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return oidc.OpenIDConfigurationForIssuer(issuer, issuer+OIDCJWKWURI), nil
}

// jwksOIDC returns all public keys used to sign JWT tokens for this cluster.
func (h *Handler) jwksOIDC(_ http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	return h.jwks(r.Context(), types.OIDCIdPCA)
}

func (h *Handler) jwks(ctx context.Context, caType types.CertAuthType) (*JWKSResponse, error) {
	clusterName, err := h.GetProxyClient().GetDomainName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Fetch the JWT public keys only.
	ca, err := h.GetProxyClient().GetCertAuthority(ctx, types.CertAuthID{
		Type:       caType,
		DomainName: clusterName,
	}, false /* loadKeys */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pairs := ca.GetTrustedJWTKeyPairs()

	// Create response and allocate space for the keys.
	var resp JWKSResponse
	resp.Keys = make([]jwt.JWK, 0, len(pairs))

	// Loop over and all add public keys in JWK format.
	for _, key := range pairs {
		jwk, err := jwt.MarshalJWK(key.PublicKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resp.Keys = append(resp.Keys, jwk)
	}
	return &resp, nil
}

// thumbprint returns the thumbprint as required by AWS when adding an OIDC Identity Provider.
// This is documented here:
// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_create_oidc_verify-thumbprint.html
// Returns the thumbprint of the top intermediate CA that signed the TLS cert used to serve HTTPS requests.
// In case of a self signed certificate, then it returns the thumbprint of the TLS cert itself.
func (h *Handler) thumbprint(_ http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	return awsoidc.ThumbprintIdP(r.Context(), h.PublicProxyAddr())
}
