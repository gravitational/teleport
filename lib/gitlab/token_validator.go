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

package gitlab

import (
	"context"
	"fmt"
	"time"

	"github.com/coreos/go-oidc"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/jwt"
)

type clusterNameGetter interface {
	GetClusterName(ctx context.Context) (types.ClusterName, error)
}

type IDTokenValidatorConfig struct {
	// Clock is used by the validator when checking expiry and issuer times of
	// tokens. If omitted, a real clock will be used.
	Clock clockwork.Clock
	// ClusterNameGetter is used to get the cluster name in order to identify
	// the correct audience for the token.
	ClusterNameGetter clusterNameGetter
	// insecure configures the validator to use HTTP rather than HTTPS. This
	// is not exported as this is only used in the test for now.
	insecure bool
}

type IDTokenValidator struct {
	IDTokenValidatorConfig
}

func NewIDTokenValidator(
	cfg IDTokenValidatorConfig,
) (*IDTokenValidator, error) {
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	if cfg.ClusterNameGetter == nil {
		return nil, trace.BadParameter(
			"ClusterNameGetter must be configured",
		)
	}

	return &IDTokenValidator{
		IDTokenValidatorConfig: cfg,
	}, nil
}

func (id *IDTokenValidator) issuerURL(domain string) string {
	scheme := "https"
	if id.insecure {
		scheme = "http"
	}

	return fmt.Sprintf("%s://%s", scheme, domain)
}

func (id *IDTokenValidator) Validate(
	ctx context.Context, domain string, token string,
) (*IDTokenClaims, error) {
	p, err := oidc.NewProvider(
		ctx,
		id.issuerURL(domain),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterNameResource, err := id.ClusterNameGetter.GetClusterName(ctx)
	if err != nil {
		return nil, err
	}

	verifier := p.Verifier(&oidc.Config{
		ClientID: clusterNameResource.GetClusterName(),
		Now:      id.Clock.Now,
	})

	idToken, err := verifier.Verify(ctx, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// `go-oidc` does not implement not before check, so we need to manually
	// perform this
	if err := jwt.CheckNotBefore(
		id.Clock.Now(), time.Minute*2, idToken,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	claims := IDTokenClaims{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, trace.Wrap(err)
	}
	return &claims, nil
}
