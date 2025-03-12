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

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// providerTimeout is the maximum time allowed to fetch provider metadata before
// giving up.
const providerTimeout = 15 * time.Second

// IDTokenValidator validates a Bitbucket issued ID Token.
type IDTokenValidator struct {
	// clock is used by the validator when checking expiry and issuer times of
	// tokens. If omitted, a real clock will be used.
	clock clockwork.Clock
}

// NewIDTokenValidator returns an initialized IDTokenValidator
func NewIDTokenValidator(clock clockwork.Clock) *IDTokenValidator {
	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	return &IDTokenValidator{
		clock: clock,
	}
}

// Validate validates a Bitbucket issued ID token.
func (id *IDTokenValidator) Validate(
	ctx context.Context, issuerURL, audience, token string,
) (*IDTokenClaims, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()

	p, err := oidc.NewProvider(timeoutCtx, issuerURL)
	if err != nil {
		return nil, trace.Wrap(err, "creating oidc provider")
	}

	verifier := p.Verifier(&oidc.Config{
		ClientID: audience,
		Now:      id.clock.Now,
	})

	idToken, err := verifier.Verify(timeoutCtx, token)
	if err != nil {
		return nil, trace.Wrap(err, "verifying token")
	}

	var claims IDTokenClaims
	if err := idToken.Claims(&claims); err != nil {
		return nil, trace.Wrap(err)
	}
	return &claims, nil
}
