/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package web

import (
	"net/http"
	"strings"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	apiscopes "github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/utils/oidc"
)

const (
	// OIDCJWKWURI is the relative path where the OIDC IdP JWKS is located
	OIDCJWKWURI = "/.well-known/jwks-oidc"
	// OktaJWKSWellknownURI is the relative path where the Okta JWKS is located
	OktaJWKSWellknownURI = "/.well-known/jwks-okta"
	// openIDConfigurationSuffix is the path of an issuer's OIDC discovery document, relative to the issuer.
	openIDConfigurationSuffix = "/.well-known/openid-configuration"
)

// openidConfiguration returns the openid-configuration for setting up the AWS OIDC Integration
func (h *Handler) openidConfiguration(_ http.ResponseWriter, _ *http.Request, _ httprouter.Params) (interface{}, error) {
	issuer, err := oidc.IssuerFromPublicAddress(h.cfg.PublicProxyAddr, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return oidc.OpenIDConfigurationForIssuer(issuer, issuer+OIDCJWKWURI), nil
}

// scopedOpenidConfiguration serves the OIDC discovery document of a scoped AWS OIDC
// integration at <issuer>/.well-known/openid-configuration, where the issuer path is
// /scope/<scope segments>/<integration name> (see awsoidc.ScopedIssuerPath).
//
// AWS fetches this endpoint anonymously, so the handler parses the scope and name from
// the path and resolves an exact integration.
// It serves a document only when the scoped AWS OIDC integration exists.
// Every other path under /scope/ returns a 404.
func (h *Handler) scopedOpenidConfiguration(_ http.ResponseWriter, r *http.Request, _ httprouter.Params) (any, error) {
	// Parse the escaped path so an encoded slash (%2F) cannot pose as a scope separator.
	scope, name, ok := parseScopedDiscoveryPath(r.URL.EscapedPath())
	if !ok {
		return nil, trace.NotFound("not found")
	}

	ig, err := h.GetProxyClient().GetIntegration(r.Context(), integrationv1.GetIntegrationRequest_builder{
		Scope: scope,
		Name:  name,
	}.Build())
	if err != nil {
		return nil, trace.NotFound("not found")
	}
	if ig.GetSubKind() != types.IntegrationSubKindAWSOIDC {
		return nil, trace.NotFound("not found")
	}

	issuerPath, err := awsoidc.ScopedIssuerPath(ig.GetScope(), ig.GetName())
	if err != nil {
		return nil, trace.NotFound("not found")
	}
	issuer, err := oidc.IssuerFromPublicAddress(h.cfg.PublicProxyAddr, issuerPath)
	if err != nil {
		return nil, trace.NotFound("not found")
	}

	// The signing keys are the cluster-wide OIDC IdP CA key set, so the document points
	// at the existing cluster JWKS endpoint rather than a per-scope path.
	clusterIssuer, err := oidc.IssuerFromPublicAddress(h.cfg.PublicProxyAddr, "")
	if err != nil {
		return nil, trace.NotFound("not found")
	}
	return oidc.OpenIDConfigurationForIssuer(issuer, clusterIssuer+OIDCJWKWURI), nil
}

// parseScopedDiscoveryPath splits a scoped discovery request path,
// /scope/<scope segments>/<integration name>/.well-known/openid-configuration, into the
// integration's scope and name. The last segment is the name and any earlier segments
// form the scope.
func parseScopedDiscoveryPath(requestPath string) (scope, name string, ok bool) {
	issuerPath, found := strings.CutSuffix(requestPath, openIDConfigurationSuffix)
	if !found {
		return "", "", false
	}

	rest, found := strings.CutPrefix(issuerPath, awsoidc.ScopedIssuerPrefix+"/")
	if !found {
		return "", "", false
	}

	segments := strings.Split(rest, "/")
	name = segments[len(segments)-1]
	if err := apiscopes.StrongValidateResourceName(name); err != nil {
		return "", "", false
	}

	scope = apiscopes.Root
	if scopeSegments := segments[:len(segments)-1]; len(scopeSegments) > 0 {
		scope = "/" + strings.Join(scopeSegments, "/")
		if err := apiscopes.StrongValidate(scope); err != nil {
			return "", "", false
		}
	}

	return scope, name, true
}

// jwksOIDC returns all public keys used to sign JWT tokens for this cluster.
func (h *Handler) jwksOIDC(_ http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	return h.jwks(r.Context(), types.OIDCIdPCA, true)
}

// thumbprint returns the thumbprint as required by AWS when adding an OIDC Identity Provider.
// This is documented here:
// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_create_oidc_verify-thumbprint.html
// Returns the thumbprint of the top intermediate CA that signed the TLS cert used to serve HTTPS requests.
// In case of a self signed certificate, then it returns the thumbprint of the TLS cert itself.
func (h *Handler) thumbprint(_ http.ResponseWriter, r *http.Request, _ httprouter.Params) (interface{}, error) {
	return awsoidc.ThumbprintIdP(r.Context(), h.PublicProxyAddr())
}
