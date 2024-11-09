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

package bitbucket

import (
	"context"
	"time"

	"github.com/coreos/go-oidc"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// IDTokenValidatorConfig contains the configuration options needed to control
// the behavior of IDTokenValidator.
type IDTokenValidatorConfig struct {
	// Clock is used by the validator when checking expiry and issuer times of
	// tokens. If omitted, a real clock will be used.
	Clock clockwork.Clock
}

// IDTokenValidator validates a Bitbucket issued ID Token.
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

// Validate validates a Bitbucket issued ID token.
func (id *IDTokenValidator) Validate(
	ctx context.Context, issuerURL, audience, token string,
) (*IDTokenClaims, error) {
	p, err := oidc.NewProvider(ctx, issuerURL)
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
