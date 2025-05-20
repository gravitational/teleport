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
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v3"
	josejwt "github.com/go-jose/go-jose/v3/jwt"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/zitadel/oidc/v3/pkg/client"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/gravitational/teleport/api/types"
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
	clusterNameResource, err := id.ClusterNameGetter.GetClusterName(ctx)
	if err != nil {
		return nil, err
	}

	audience := clusterNameResource.GetClusterName()
	issuer := id.issuerURL(domain)

	// TODO(noah): It'd be nice to cache the OIDC discovery document fairly
	// aggressively across join tokens since this isn't going to change very
	// regularly.
	dc, err := client.Discover(ctx, issuer, otelhttp.DefaultClient)
	if err != nil {
		return nil, trace.Wrap(err, "discovering oidc document")
	}

	// TODO(noah): Ideally we'd cache the remote keyset across joins/join tokens
	// based on the issuer.
	ks := rp.NewRemoteKeySet(otelhttp.DefaultClient, dc.JwksURI)
	verifier := rp.NewIDTokenVerifier(issuer, audience, ks)
	// TODO(noah): It'd be ideal if we could extend the verifier to use an
	// injected "now" time.

	claims, err := rp.VerifyIDToken[*IDTokenClaims](ctx, token, verifier)
	if err != nil {
		return nil, trace.Wrap(err, "verifying token")
	}

	return claims, nil
}

// ValidateTokenWithJWKS validates a token using the provided JWKS data.
// Used in cases where GitLab is not reachable from the Teleport cluster.
func (id *IDTokenValidator) ValidateTokenWithJWKS(
	ctx context.Context,
	jwksData []byte,
	token string,
) (*IDTokenClaims, error) {
	parsed, err := josejwt.ParseSigned(token)
	if err != nil {
		return nil, trace.Wrap(err, "parsing jwt")
	}

	jwks := jose.JSONWebKeySet{}
	if err := json.Unmarshal(jwksData, &jwks); err != nil {
		return nil, trace.Wrap(err, "parsing provided jwks")
	}

	stdClaims := josejwt.Claims{}
	if err := parsed.Claims(jwks, &stdClaims); err != nil {
		return nil, trace.Wrap(err, "validating jwt signature")
	}

	clusterNameResource, err := id.ClusterNameGetter.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "getting cluster name")
	}

	leeway := time.Second * 10
	err = stdClaims.ValidateWithLeeway(josejwt.Expected{
		Audience: []string{
			clusterNameResource.GetClusterName(),
		},
		Time: id.Clock.Now(),
	}, leeway)
	if err != nil {
		return nil, trace.Wrap(err, "validating standard claims")
	}

	claims := IDTokenClaims{}
	if err := parsed.Claims(jwks, &claims); err != nil {
		return nil, trace.Wrap(err, "validating custom claims")
	}

	return &claims, nil
}
