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

package integrationv1

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
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

	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}
	return s.generateAWSOIDCTokenWithoutAuthZ(ctx)
}

// generateAWSOIDCTokenWithoutAuthZ generates a token to be used when executing an AWS OIDC Integration action.
// Bypasses authz and should only be used by other methods that validate AuthZ.
func (s *Service) generateAWSOIDCTokenWithoutAuthZ(ctx context.Context) (*integrationpb.GenerateAWSOIDCTokenResponse, error) {
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

func (s *AWSOIDCService) awsClientReq(ctx context.Context, integrationName, region string) (*awsoidc.AWSClientRequest, error) {
	integration, err := s.integrationService.GetIntegration(ctx, &integrationpb.GetIntegrationRequest{
		Name: integrationName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if integration.GetSubKind() != types.IntegrationSubKindAWSOIDC {
		return nil, trace.BadParameter("integration subkind (%s) mismatch", integration.GetSubKind())
	}

	awsoidcSpec := integration.GetAWSOIDCIntegrationSpec()
	if awsoidcSpec == nil {
		return nil, trace.BadParameter("missing spec fields for %q (%q) integration", integration.GetName(), integration.GetSubKind())
	}

	token, err := s.integrationService.generateAWSOIDCTokenWithoutAuthZ(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &awsoidc.AWSClientRequest{
		IntegrationName: integrationName,
		Token:           token.Token,
		RoleARN:         integration.GetAWSOIDCIntegrationSpec().RoleARN,
		Region:          region,
	}, nil
}

// ListIntegrations returns a paginated list of Databases.
func (s *AWSOIDCService) ListDatabases(ctx context.Context, req *integrationpb.ListDatabasesRequest) (*integrationpb.ListDatabasesResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}

	awsClientReq, err := s.awsClientReq(ctx, req.Integration, req.Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listDBsClient, err := awsoidc.NewListDatabasesClient(ctx, awsClientReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listDBsResp, err := awsoidc.ListDatabases(ctx, listDBsClient, awsoidc.ListDatabasesRequest{
		Region:    req.Region,
		RDSType:   req.RdsType,
		Engines:   req.Engines,
		NextToken: req.NextToken,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dbList := make([]*types.DatabaseV3, 0, len(listDBsResp.Databases))
	for _, db := range listDBsResp.Databases {
		dbV3, ok := db.(*types.DatabaseV3)
		if !ok {
			return nil, trace.BadParameter("unsupported database type %T", db)
		}
		dbList = append(dbList, dbV3)
	}

	return &integrationpb.ListDatabasesResponse{
		Databases: dbList,
		NextToken: listDBsResp.NextToken,
	}, nil
}
