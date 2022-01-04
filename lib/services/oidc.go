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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
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

	if oc.GetGoogleServiceAccountURI() == "" && oc.GetGoogleServiceAccount() == "" {
		return nil
	}

	if oc.GetGoogleServiceAccountURI() != "" && oc.GetGoogleServiceAccount() != "" {
		return trace.BadParameter("at most one of google_service_account_uri and google_service_account can be specified, not both")
	}

	if oc.GetGoogleServiceAccountURI() != "" {
		uri, err := utils.ParseSessionsURI(oc.GetGoogleServiceAccountURI())
		if err != nil {
			return trace.Wrap(err)
		}
		if uri.Scheme != teleport.SchemeFile {
			return trace.BadParameter("only %v:// scheme is supported for google_service_account_uri", teleport.SchemeFile)
		}
	}

	if oc.GetGoogleAdminEmail() == "" && !oc.GetGoogleTransitiveGroups() {
		return trace.BadParameter("whenever google_service_account_uri or google_service_account are specified and google_transitive_groups is not set, google_admin_email should be specified as well, read https://developers.google.com/identity/protocols/OAuth2ServiceAccount#delegatingauthority for more details")
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

// OIDCClaimsToTraits converts OIDC-style claims into teleport-specific trait format
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

// UnmarshalOIDCConnector unmarshals the OIDCConnector resource from JSON.
func UnmarshalOIDCConnector(bytes []byte, opts ...MarshalOption) (types.OIDCConnector, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	err = utils.FastUnmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V2:
		var c types.OIDCConnectorV2
		if err := utils.FastUnmarshal(bytes, &c); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := ValidateOIDCConnector(&c); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			c.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			c.SetExpiry(cfg.Expires)
		}
		return &c, nil
	}

	return nil, trace.BadParameter("OIDC connector resource version %v is not supported", h.Version)
}

// MarshalOIDCConnector marshals the OIDCConnector resource to JSON.
func MarshalOIDCConnector(oidcConnector types.OIDCConnector, opts ...MarshalOption) ([]byte, error) {
	if err := ValidateOIDCConnector(oidcConnector); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch oidcConnector := oidcConnector.(type) {
	case *types.OIDCConnectorV2:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *oidcConnector
			copy.SetResourceID(0)
			oidcConnector = &copy
		}
		return utils.FastMarshal(oidcConnector)
	default:
		return nil, trace.BadParameter("unrecognized OIDC connector version %T", oidcConnector)
	}
}
