/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package integrationv1

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
)

// defaultTokenTTL is the default TTL for AWS OIDC tokens
const defaultTokenTTL = time.Minute

// GenerateAWSOIDCToken generates a token to be used when executing an AWS OIDC Integration action.
func (s *Service) GenerateAWSOIDCToken(ctx context.Context, req *integrationpb.GenerateAWSOIDCTokenRequest) (*integrationpb.GenerateAWSOIDCTokenResponse, error) {
	_, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, true, types.KindIntegration, types.VerbUse)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	username, err := authz.GetClientUsername(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := GenerateAWSOIDCToken(ctx, AWSOIDCTokenConfig{
		CAGetter: s.caGetter,
		Clock:    s.clock,
		Issuer:   req.Issuer,
		Username: username,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &integrationpb.GenerateAWSOIDCTokenResponse{
		Token: token,
	}, nil
}

// AWSOIDCTokenConfig contains configuration to be used when generating an AWS OIDC token.
type AWSOIDCTokenConfig struct {
	CAGetter CAGetter
	Clock    clockwork.Clock
	// TTL is the time to live for the token
	TTL time.Duration
	// Issuer is the issuer of the token.
	Issuer string
	// Username is the Teleport identity.
	Username string
}

func (c *AWSOIDCTokenConfig) checkAndSetDefaults() error {
	if c.CAGetter == nil {
		return trace.BadParameter("ca getter is required")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.TTL == 0 {
		c.TTL = defaultTokenTTL
	}
	if c.Issuer == "" {
		return trace.BadParameter("issuer is required")
	}
	return nil
}

// GenerateToken generates a token to be used when executing an AWS OIDC Integration action.
func GenerateAWSOIDCToken(ctx context.Context, config AWSOIDCTokenConfig) (string, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return "", trace.Wrap(err)
	}

	clusterName, err := config.CAGetter.GetDomainName()
	if err != nil {
		return "", trace.Wrap(err)
	}

	ca, err := config.CAGetter.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.OIDCIdPCA,
		DomainName: clusterName,
	}, true)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Extract the JWT signing key and sign the claims.
	signer, err := config.CAGetter.GetKeyStore().GetJWTSigner(ctx, ca)
	if err != nil {
		return "", trace.Wrap(err)
	}

	privateKey, err := services.GetJWTSigner(signer, ca.GetClusterName(), config.Clock)
	if err != nil {
		return "", trace.Wrap(err)
	}

	token, err := privateKey.SignAWSOIDC(jwt.SignParams{
		Username: config.Username,
		Audience: types.IntegrationAWSOIDCAudience,
		Subject:  types.IntegrationAWSOIDCSubject,
		Issuer:   config.Issuer,
		Expires:  config.Clock.Now().Add(config.TTL),
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	return token, nil
}
