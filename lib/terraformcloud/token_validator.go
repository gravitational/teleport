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
	"time"

	"github.com/coreos/go-oidc"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/jwt"
)

// DefaultIssuerURL is the issuer URL for Terraform Cloud
const DefaultIssuerURL = "https://app.terraform.io"

// IDTokenValidatorConfig contains the configuration options needed to control
// the behavior of IDTokenValidator.
type IDTokenValidatorConfig struct {
	// Clock is used by the validator when checking expiry and issuer times of
	// tokens. If omitted, a real clock will be used.
	Clock clockwork.Clock
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
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

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
	p, err := oidc.NewProvider(
		ctx,
		id.issuerURL(hostname),
	)
	if err != nil {
		return nil, trace.Wrap(err, "creating oidc provider")
	}

	verifier := p.Verifier(&oidc.Config{
		ClientID: audience,
		Now:      id.Clock.Now,
	})

	idToken, err := verifier.Verify(ctx, token)
	if err != nil {
		return nil, trace.Wrap(err, "verifying token")
	}

	// `go-oidc` does not implement not before check, so we need to manually
	// perform this
	if err := jwt.CheckNotBefore(
		id.Clock.Now(), time.Minute*2, idToken,
	); err != nil {
		return nil, trace.Wrap(err, "enforcing nbf")
	}

	claims := IDTokenClaims{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, trace.Wrap(err)
	}
	return &claims, nil
}
