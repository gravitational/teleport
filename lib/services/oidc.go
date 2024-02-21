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

package services

import (
	"net/url"

	"github.com/coreos/go-oidc/jose"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

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

// GetRedirectURL gets a redirect URL for the given connector. If the connector
// has a redirect URL which matches the host of the given Proxy address, then
// that one will be returned. Otherwise, the first URL in the list will be returned.
func GetRedirectURL(conn types.OIDCConnector, proxyAddr string) (string, error) {
	if len(conn.GetRedirectURLs()) == 0 {
		return "", trace.BadParameter("No redirect URLs provided")
	}

	// If a specific proxyAddr wasn't provided in the oidc auth request,
	// or there is only one redirect URL, use the first redirect URL.
	if proxyAddr == "" || len(conn.GetRedirectURLs()) == 1 {
		return conn.GetRedirectURLs()[0], nil
	}

	proxyNetAddr, err := utils.ParseAddr(proxyAddr)
	if err != nil {
		return "", trace.Wrap(err, "invalid proxy address %v", proxyAddr)
	}

	var matchingHostname string
	for _, r := range conn.GetRedirectURLs() {
		redirectURL, err := url.ParseRequestURI(r)
		if err != nil {
			return "", trace.Wrap(err)
		}

		// If we have a direct host:port match, return it.
		if proxyNetAddr.String() == redirectURL.Host {
			return r, nil
		}

		// If we have a matching host, but not port,
		// save it as the best match for now.
		if matchingHostname == "" && proxyNetAddr.Host() == redirectURL.Hostname() {
			matchingHostname = r
		}
	}

	if matchingHostname != "" {
		return matchingHostname, nil
	}

	// No match, default to the first redirect URL.
	return conn.GetRedirectURLs()[0], nil
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
	// V2 and V3 have the same layout, the only change is in the behavior
	case types.V2, types.V3:
		var c types.OIDCConnectorV3
		if err := utils.FastUnmarshal(bytes, &c); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := c.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			c.SetResourceID(cfg.ID)
		}
		if cfg.Revision != "" {
			c.SetRevision(cfg.Revision)
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
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch oidcConnector := oidcConnector.(type) {
	case *types.OIDCConnectorV3:
		if err := oidcConnector.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoResourceID(cfg.PreserveResourceID, oidcConnector))
	default:
		return nil, trace.BadParameter("unrecognized OIDC connector version %T", oidcConnector)
	}
}
