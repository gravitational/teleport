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

package githubactions

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v3"
	josejwt "github.com/go-jose/go-jose/v3/jwt"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/oidc"
)

const audience = "teleport.cluster.local"

type IDTokenValidatorConfig struct {
	// GitHubIssuerHost is the host of the Issuer for tokens issued by
	// GitHub's cloud hosted version. If no GHESHost override is provided to
	// the call to Validate, then this will be used as the host.
	GitHubIssuerHost string
	// insecure configures the validator to use HTTP rather than HTTPS. This
	// is not exported as this is only used in the test for now.
	insecure bool
}

type IDTokenValidator struct {
	IDTokenValidatorConfig
}

func NewIDTokenValidator(cfg IDTokenValidatorConfig) *IDTokenValidator {
	if cfg.GitHubIssuerHost == "" {
		cfg.GitHubIssuerHost = DefaultIssuerHost
	}

	return &IDTokenValidator{
		IDTokenValidatorConfig: cfg,
	}
}

func (id *IDTokenValidator) issuerURL(
	GHESHost string, enterpriseSlug string,
) string {
	scheme := "https"
	if id.insecure {
		scheme = "http"
	}

	if GHESHost == "" {
		url := fmt.Sprintf("%s://%s", scheme, id.GitHubIssuerHost)
		// Support custom enterprise slugs, as per:
		// https://docs.github.com/en/enterprise-cloud@latest/actions/deployment/security-hardening-your-deployments/about-security-hardening-with-openid-connect#customizing-the-issuer-value-for-an-enterprise
		if enterpriseSlug != "" {
			url = fmt.Sprintf("%s/%s", url, enterpriseSlug)
		}
		return url
	}
	return fmt.Sprintf("%s://%s/_services/token", scheme, GHESHost)
}

func (id *IDTokenValidator) Validate(
	ctx context.Context, GHESHost string, enterpriseSlug string, token string,
) (*IDTokenClaims, error) {
	issuer := id.issuerURL(GHESHost, enterpriseSlug)
	return oidc.ValidateToken[*IDTokenClaims](ctx, issuer, audience, token)
}

// ValidateTokenWithJWKS validates a GitHub Actions JWT using a configured
// JWKS rather than fetching from well-known. This supports cases where GHES
// is not accessible to the Teleport Auth Server.
func ValidateTokenWithJWKS(
	now time.Time,
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

	leeway := time.Second * 10
	err = stdClaims.ValidateWithLeeway(josejwt.Expected{
		Audience: []string{
			audience,
		},
		Time: now,
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
