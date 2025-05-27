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

package terraformcloud

import (
	"context"
	"fmt"

	"github.com/gravitational/teleport/lib/oidc"
)

// DefaultIssuerURL is the issuer URL for Terraform Cloud
const DefaultIssuerURL = "https://app.terraform.io"

// IDTokenValidatorConfig contains the configuration options needed to control
// the behavior of IDTokenValidator.
type IDTokenValidatorConfig struct {
	// issuerHostnameOverride overrides the default Terraform Cloud issuer URL. Used only in
	// tests.
	issuerHostnameOverride string
	// insecure configures the validator to use HTTP rather than HTTPS. This
	// is not exported as this is only used in the test for now.
	insecure bool
}

// IDTokenValidator validates a Spacelift issued ID Token.
type IDTokenValidator struct {
	IDTokenValidatorConfig
}

// NewIDTokenValidator returns an initialized IDTokenValidator
func NewIDTokenValidator(
	cfg IDTokenValidatorConfig,
) *IDTokenValidator {
	return &IDTokenValidator{
		IDTokenValidatorConfig: cfg,
	}
}

// issuerURL returns an issuer URL. If overridden by tests with
// `issuerHostnameOverride`, returns that value. Otherwise, if an issuer is
// provided via the token (for TFE), returns that, else returns the default
// issuer for Terraform Cloud.
func (id *IDTokenValidator) issuerURL(tfeHostname string) string {
	if id.issuerHostnameOverride == "" && tfeHostname == "" {
		return DefaultIssuerURL
	}

	hostname := tfeHostname
	if id.issuerHostnameOverride != "" {
		hostname = id.issuerHostnameOverride
	}

	scheme := "https"
	if id.insecure {
		scheme = "http"
	}

	return fmt.Sprintf("%s://%s", scheme, hostname)
}

// Validate validates a Terraform issued ID token.
func (id *IDTokenValidator) Validate(
	ctx context.Context, audience, hostname, token string,
) (*IDTokenClaims, error) {
	issuer := id.issuerURL(hostname)
	return oidc.ValidateToken[*IDTokenClaims](ctx, issuer, audience, token)
}
