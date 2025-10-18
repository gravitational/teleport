/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package env0

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/oidc"
)

const (
	// env0IssuerURL is the env0 public issuer URL
	env0IssuerURL = "https://login.app.env0.com/"

	// env0Audience is the audience for the token. This is unfortunately hard
	// coded.
	env0Audience = "https://app.env0.com"
)

// IDTokenValidator can be used to validate env0 OIDC tokens.
type IDTokenValidator struct {
	validator *oidc.CachingTokenValidator[*IDTokenClaims]
}

// ValidateToken validates an env0 OIDC token using a remote, cached OIDC
// endpoint
func (v *IDTokenValidator) ValidateToken(
	ctx context.Context,
	token []byte,
) (*IDTokenClaims, error) {
	validator, err := v.validator.GetValidator(ctx, env0IssuerURL, env0Audience)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	claims, err := validator.ValidateToken(ctx, string(token))
	if err != nil {
		return nil, trace.Wrap(err, "validating OIDC token")
	}

	return claims, nil
}

// NewOIDCTokenValidator constructs a KubernetesOIDCTokenValidator.
func NewIDTokenValidator() (*IDTokenValidator, error) {
	validator, err := oidc.NewCachingTokenValidator[*IDTokenClaims](clockwork.NewRealClock())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &IDTokenValidator{
		validator: validator,
	}, nil
}
