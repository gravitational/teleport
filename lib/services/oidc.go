/*
Copyright 2021 Gravitational, Inc.

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

package services

import (
	"net/url"

	"github.com/coreos/go-oidc/jose"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// ValidateOIDCConnector validates the OIDC connector and sets default values
func ValidateOIDCConnector(oc types.OIDCConnector) error {
	if err := oc.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if _, err := url.Parse(oc.GetIssuerURL()); err != nil {
		return trace.BadParameter("IssuerURL: bad url: '%v'", oc.GetIssuerURL())
	}
	if _, err := url.Parse(oc.GetRedirectURL()); err != nil {
		return trace.BadParameter("RedirectURL: bad url: '%v'", oc.GetRedirectURL())
	}
	if oc.GetGoogleServiceAccountURI() != "" {
		uri, err := utils.ParseSessionsURI(oc.GetGoogleServiceAccountURI())
		if err != nil {
			return trace.Wrap(err)
		}
		if uri.Scheme != teleport.SchemeFile {
			return trace.BadParameter("only %v:// scheme is supported for google_service_account_uri", teleport.SchemeFile)
		}
		if oc.GetGoogleAdminEmail() == "" {
			return trace.BadParameter("whenever google_service_account_uri is specified, google_admin_email should be set as well, read https://developers.google.com/identity/protocols/OAuth2ServiceAccount#delegatingauthority for more details")
		}
	}
	return nil
}

// GetClaimNames returns a list of claim names from the claim values
func GetClaimNames(claims jose.Claims) []string {
	var out []string
	for claim := range claims {
		out = append(out, claim)
	}
	return out
}

// OIDCClaimsToTraits converts OIDC-style claims into the standardized
// teleport trait format.
func OIDCClaimsToTraits(claims jose.Claims) map[string][]string {
	traits := make(map[string][]string)

	for claimName := range claims {
		claimValue, ok, _ := claims.StringClaim(claimName)
		if ok {
			traits[claimName] = []string{claimValue}
		}
		claimValues, ok, _ := claims.StringsClaim(claimName)
		if ok {
			traits[claimName] = claimValues
		}
	}

	return traits
}
