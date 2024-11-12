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

package oidc

// OpenIDConfiguration is the default OpenID Configuration used by Teleport.
// Based on https://openid.net/specs/openid-connect-discovery-1_0.html
type OpenIDConfiguration struct {
	Issuer                           string   `json:"issuer"`
	JWKSURI                          string   `json:"jwks_uri"`
	Claims                           []string `json:"claims"`
	IdTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
	ResponseTypesSupported           []string `json:"response_types_supported"`
	ScopesSupported                  []string `json:"scopes_supported,omitempty"`
	SubjectTypesSupported            []string `json:"subject_types_supported,omitempty"`
}

// OpenIDConfigurationForIssuer returns the OpenID Configuration for
// the given issuer and JWKS URI.
func OpenIDConfigurationForIssuer(issuer, jwksURI string) OpenIDConfiguration {
	return OpenIDConfiguration{
		Issuer:                           issuer,
		JWKSURI:                          jwksURI,
		Claims:                           []string{"iss", "sub", "obo", "aud", "jti", "iat", "exp", "nbf"},
		IdTokenSigningAlgValuesSupported: []string{"RS256"},
		ResponseTypesSupported:           []string{"id_token"},
		ScopesSupported:                  []string{"openid"},
		SubjectTypesSupported:            []string{"public", "pair-wise"},
	}
}
