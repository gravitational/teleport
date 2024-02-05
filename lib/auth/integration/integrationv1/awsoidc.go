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
	"github.com/sirupsen/logrus"

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/oidc"
)

// GenerateAWSOIDCToken generates a token to be used when executing an AWS OIDC Integration action.
func (s *Service) GenerateAWSOIDCToken(ctx context.Context, _ *integrationpb.GenerateAWSOIDCTokenRequest) (*integrationpb.GenerateAWSOIDCTokenResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(true, types.KindIntegration, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}

	username, err := authz.GetClientUsername(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName, err := s.cache.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ca, err := s.cache.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.OIDCIdPCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Extract the JWT signing key and sign the claims.
	signer, err := s.keyStoreManager.GetJWTSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	privateKey, err := services.GetJWTSigner(signer, ca.GetClusterName(), s.clock)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	issuer, err := oidc.IssuerForCluster(ctx, s.cache)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := privateKey.SignAWSOIDC(jwt.SignParams{
		Username: username,
		Audience: types.IntegrationAWSOIDCAudience,
		Subject:  types.IntegrationAWSOIDCSubject,
		Issuer:   issuer,
		// Token expiration is not controlled by the Expires property.
		// It is defined by assumed IAM Role's "Maximum session duration" (usually 1h).
		Expires: s.clock.Now().Add(time.Minute),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &integrationpb.GenerateAWSOIDCTokenResponse{
		Token: token,
	}, nil
}

// AWSOIDCServiceConfig holds configuration options for the AWSOIDC Integration gRPC service.
type AWSOIDCServiceConfig struct {
	IntegrationService *Service
	Authorizer         authz.Authorizer
	Logger             *logrus.Entry
}

// CheckAndSetDefaults checks the AWSOIDCServiceConfig fields and returns an error if a required param is not provided.
// Authorizer and IntegrationService are required params.
func (s *AWSOIDCServiceConfig) CheckAndSetDefaults() error {
	if s.Authorizer == nil {
		return trace.BadParameter("authorizer is required")
	}

	if s.IntegrationService == nil {
		return trace.BadParameter("integration service is required")
	}

	if s.Logger == nil {
		s.Logger = logrus.WithField(trace.Component, "integrations.awsoidc.service")
	}

	return nil
}

// AWSOIDCService implements the teleport.integration.v1.AWSOIDCService RPC service.
type AWSOIDCService struct {
	integrationpb.UnimplementedAWSOIDCServiceServer

	integrationService *Service
	authorizer         authz.Authorizer
	logger             *logrus.Entry
}

// NewAWSOIDCService returns a new AWSOIDCService.
func NewAWSOIDCService(cfg *AWSOIDCServiceConfig) (*AWSOIDCService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &AWSOIDCService{
		integrationService: cfg.IntegrationService,
		logger:             cfg.Logger,
		authorizer:         cfg.Authorizer,
	}, nil
}

var _ integrationpb.AWSOIDCServiceServer = (*AWSOIDCService)(nil)

// ListIntegrations returns a paginated list of Databases.
func (s *AWSOIDCService) ListDatabases(ctx context.Context, req *integrationpb.ListDatabasesRequest) (*integrationpb.ListDatabasesResponse, error) {
	_, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, true, types.KindIntegration, types.VerbUse)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return nil, trace.NotImplemented("not implemented")
}
