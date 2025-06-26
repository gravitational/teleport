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

package integrationv1

import (
	"context"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/integrations/awsra"
)

// GenerateAWSRACredentials generates a set of AWS credentials which uses the AWS Roles Anywhere integration.
func (s *Service) GenerateAWSRACredentials(ctx context.Context, req *integrationpb.GenerateAWSRACredentialsRequest) (*integrationpb.GenerateAWSRACredentialsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, allowedRole := range []types.SystemRole{types.RoleAuth, types.RoleProxy} {
		if authz.HasBuiltinRole(*authCtx, string(allowedRole)) {
			awsCredentials, err := s.generateAWSRACredentialsWithoutAuthZ(ctx, &generateAWSRolesAnywhereCredentialsRequest{
				integration:                   req.GetIntegration(),
				profileARN:                    req.GetProfileArn(),
				roleARN:                       req.GetRoleArn(),
				profileAcceptsRoleSessionName: req.GetProfileAcceptsRoleSessionName(),
				subjectName:                   req.GetSubjectName(),
				sessionMaxDuration:            req.GetSessionMaxDuration(),
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &integrationpb.GenerateAWSRACredentialsResponse{
				AccessKeyId:     awsCredentials.accessKeyId,
				SecretAccessKey: awsCredentials.secretAccessKey,
				SessionToken:    awsCredentials.sessionToken,
				Expiration:      timestamppb.New(awsCredentials.expiration),
			}, nil
		}
	}

	return nil, trace.AccessDenied("credential generation is only available to auth or proxy services")
}

type generateAWSRolesAnywhereCredentialsRequest struct {
	integration                   string
	trustAnchorARN                string
	profileARN                    string
	profileAcceptsRoleSessionName bool
	roleARN                       string
	subjectName                   string
	sessionMaxDuration            *durationpb.Duration
}

type generateAWSRolesAnywhereCredentialsResponse struct {
	accessKeyId       string
	secretAccessKey   string
	sessionToken      string
	expiration        time.Time
	trustAnchorRegion string
}

// generateAWSRACredentialsWithoutAuthZ generates a set of AWS credentials which uses the AWS Roles Anywhere integration.
// Bypasses authz and should only be used by other methods that validate AuthZ.
func (s *Service) generateAWSRACredentialsWithoutAuthZ(ctx context.Context, req *generateAWSRolesAnywhereCredentialsRequest) (*generateAWSRolesAnywhereCredentialsResponse, error) {
	// When testing connectivity (see AWSRolesAnywherePing method), the integration name is not provided and the TrustAnchorARN comes from the request.
	trustAnchorARN := req.trustAnchorARN
	profileARN := req.profileARN
	roleARN := req.roleARN
	acceptsRoleSessionName := false // defaults to false because if the Profile does not accept role session names, the request will fail.

	if req.integration != "" {
		integration, err := s.cache.GetIntegration(ctx, req.integration)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		spec := integration.GetAWSRolesAnywhereIntegrationSpec()
		if spec == nil {
			return nil, trace.BadParameter("integration %q is not an AWSRA integration", req.integration)
		}

		trustAnchorARN = spec.TrustAnchorARN
		profileARN = spec.ProfileSyncConfig.ProfileARN
		roleARN = spec.ProfileSyncConfig.RoleARN
		acceptsRoleSessionName = spec.ProfileSyncConfig.ProfileAcceptsRoleSessionName
	}

	trustAnchorARNParsed, err := arn.Parse(trustAnchorARN)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse trust anchor ARN %q", trustAnchorARN)
	}
	trustAnchorRegion := trustAnchorARNParsed.Region

	var durationSeconds *int
	if req.sessionMaxDuration.AsDuration() != 0 {
		d := int(req.sessionMaxDuration.AsDuration().Seconds())
		durationSeconds = &d
	}

	awsCredentials, err := awsra.GenerateCredentials(ctx, awsra.GenerateCredentialsRequest{
		Clock:                 s.clock,
		TrustAnchorARN:        trustAnchorARN,
		ProfileARN:            profileARN,
		RoleARN:               roleARN,
		SubjectCommonName:     req.subjectName,
		DurationSeconds:       durationSeconds,
		AcceptRoleSessionName: acceptsRoleSessionName,
		KeyStoreManager:       s.keyStoreManager,
		Cache:                 s.cache,
		CreateSession:         s.awsRolesAnywhereCreateSessionFn,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &generateAWSRolesAnywhereCredentialsResponse{
		accessKeyId:       awsCredentials.AccessKeyID,
		secretAccessKey:   awsCredentials.SecretAccessKey,
		sessionToken:      awsCredentials.SessionToken,
		expiration:        awsCredentials.Expiration,
		trustAnchorRegion: trustAnchorRegion,
	}, nil
}

// AWSRolesAnywhereServiceConfig holds configuration options for the AWSRolesAnywhere Integration gRPC service.
type AWSRolesAnywhereServiceConfig struct {
	// IntegrationService is the service that provides access to integration credentials.
	IntegrationService *Service
	// Authorizer is the authorizer used to check access to the integration.
	Authorizer authz.Authorizer

	Clock  clockwork.Clock
	Logger *slog.Logger
}

// CheckAndSetDefaults checks the AWSRolesAnywhereServiceConfig fields and returns an error if a required param is not provided.
// Authorizer and IntegrationService are required params.
func (s *AWSRolesAnywhereServiceConfig) CheckAndSetDefaults() error {
	if s.Authorizer == nil {
		return trace.BadParameter("authorizer is required")
	}

	if s.IntegrationService == nil {
		return trace.BadParameter("integration service is required")
	}

	if s.Clock == nil {
		s.Clock = clockwork.NewRealClock()
	}

	if s.Logger == nil {
		s.Logger = slog.With(teleport.ComponentKey, "integrations.awsra.service")
	}

	return nil
}

// AWSRolesAnywhereService implements the teleport.integration.v1.AWSRolesAnywhereService RPC service.
type AWSRolesAnywhereService struct {
	integrationpb.UnimplementedAWSRolesAnywhereServiceServer

	integrationService *Service
	authorizer         authz.Authorizer
	logger             *slog.Logger
	clock              clockwork.Clock
}

// NewAWSRolesAnywhereService returns a new AWSRolesAnywhereService.
func NewAWSRolesAnywhereService(cfg *AWSRolesAnywhereServiceConfig) (*AWSRolesAnywhereService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &AWSRolesAnywhereService{
		integrationService: cfg.IntegrationService,
		logger:             cfg.Logger,
		authorizer:         cfg.Authorizer,
		clock:              cfg.Clock,
	}, nil
}

var _ integrationpb.AWSRolesAnywhereServiceServer = (*AWSRolesAnywhereService)(nil)

// ListRolesAnywhereProfiles returns a paginated list of Roles Anywhere Profiles.
func (s *AWSRolesAnywhereService) ListRolesAnywhereProfiles(ctx context.Context, req *integrationpb.ListRolesAnywhereProfilesRequest) (*integrationpb.ListRolesAnywhereProfilesResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}

	credentials, err := s.integrationService.generateAWSRACredentialsWithoutAuthZ(ctx, &generateAWSRolesAnywhereCredentialsRequest{
		integration: req.GetIntegration(),
		subjectName: authCtx.Identity.GetIdentity().Username,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listRolesAnywhereClient, err := awsra.NewListRolesAnywhereProfilesClient(ctx, &awsra.AWSClientConfig{
		Credentials: awsra.Credentials{
			AccessKeyID:     credentials.accessKeyId,
			SecretAccessKey: credentials.secretAccessKey,
			SessionToken:    credentials.sessionToken,
			Expiration:      credentials.expiration,
			Version:         1,
		},
		Region: credentials.trustAnchorRegion,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	profileList, err := awsra.ListRolesAnywhereProfiles(ctx, listRolesAnywhereClient, awsra.ListRolesAnywhereProfilesRequest{
		NextToken: req.GetNextPageToken(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &integrationpb.ListRolesAnywhereProfilesResponse{
		Profiles:      profileList.Profiles,
		NextPageToken: profileList.NextToken,
	}, nil
}

// AWSRolesAnywherePing performs a health check for the AWS Roles Anywhere integration.
// It returns the caller identity and the number of AWS Roles Anywhere Profiles that are active.
func (s *AWSRolesAnywhereService) AWSRolesAnywherePing(ctx context.Context, req *integrationpb.AWSRolesAnywherePingRequest) (*integrationpb.AWSRolesAnywherePingResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}

	credentialsRequest := &generateAWSRolesAnywhereCredentialsRequest{
		subjectName:    authCtx.Identity.GetIdentity().Username,
		integration:    req.GetIntegration(),
		trustAnchorARN: req.GetTrustAnchorArn(),
		profileARN:     req.GetProfileArn(),
		roleARN:        req.GetRoleArn(),
	}

	credentials, err := s.integrationService.generateAWSRACredentialsWithoutAuthZ(ctx, credentialsRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pingClient, err := awsra.NewPingClient(ctx, &awsra.AWSClientConfig{
		Credentials: awsra.Credentials{
			AccessKeyID:     credentials.accessKeyId,
			SecretAccessKey: credentials.secretAccessKey,
			SessionToken:    credentials.sessionToken,
			Expiration:      credentials.expiration,
			Version:         1,
		},
		Region: credentials.trustAnchorRegion,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pingResp, err := awsra.Ping(ctx, pingClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &integrationpb.AWSRolesAnywherePingResponse{
		ProfileCount: int32(pingResp.EnabledProfileCounter),
		AccountId:    pingResp.AccountID,
		Arn:          pingResp.ARN,
		UserId:       pingResp.UserID,
	}, nil
}
