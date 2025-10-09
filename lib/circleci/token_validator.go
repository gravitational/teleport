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

package circleci

import (
	"context"

	"github.com/coreos/go-oidc"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

func ValidateToken(
	ctx context.Context,
	clock clockwork.Clock,
	issuerURLTemplate string,
	organizationID string,
	token string,
) (*IDTokenClaims, error) {
	issuer := issuerURL(issuerURLTemplate, organizationID)
	// We can't cache the provider as the issuer changes for every circleci
	// organization - we should build a provider that can support a global
	// cache.
	p, err := oidc.NewProvider(
		ctx,
		issuer,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	verifier := p.Verifier(&oidc.Config{
		ClientID: organizationID,
		Now:      clock.Now,
	})

	idToken, err := verifier.Verify(ctx, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	claims := IDTokenClaims{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, trace.Wrap(err)
	}
	return &claims, nil
}
