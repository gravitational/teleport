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

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
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
			return s.generateAWSRACredentialsWithoutAuthZ(ctx, req, "" /* trustAnchor */)
		}
	}

	return nil, trace.AccessDenied("credential generation is only available to auth or proxy services")
}

// generateAWSRACredentialsWithoutAuthZ generates a set of AWS credentials which uses the AWS Roles Anywhere integration.
// Bypasses authz and should only be used by other methods that validate AuthZ.
// If trustAnchor is unset, it will use the trust anchor from the integration spec.
func (s *Service) generateAWSRACredentialsWithoutAuthZ(ctx context.Context, req *integrationpb.GenerateAWSRACredentialsRequest, trustAnchor string) (*integrationpb.GenerateAWSRACredentialsResponse, error) {
	if trustAnchor == "" {
		spec, err := s.getAWSRolesAnywhereIntegrationSpec(ctx, req.GetIntegration())
		if err != nil {
			return nil, trace.BadParameter("trust anchor not provided and could not be fetched from the integration: %v", err)
		}

		trustAnchor = spec.TrustAnchorARN
	}

	var durationSeconds *int
	if req.GetSessionMaxDuration().AsDuration() != 0 {
		d := int(req.GetSessionMaxDuration().AsDuration().Seconds())
		durationSeconds = &d
	}

	awsCredentials, err := awsra.GenerateCredentials(ctx, awsra.GenerateCredentialsRequest{
		Clock:                 s.clock,
		TrustAnchorARN:        trustAnchor,
		ProfileARN:            req.GetProfileArn(),
		RoleARN:               req.GetRoleArn(),
		SubjectCommonName:     req.GetSubjectName(),
		DurationSeconds:       durationSeconds,
		AcceptRoleSessionName: req.GetProfileAcceptsRoleSessionName(),
		KeyStoreManager:       s.keyStoreManager,
		Cache:                 s.cache,
		CreateSession:         s.awsRolesAnywhereCreateSessionFn,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &integrationpb.GenerateAWSRACredentialsResponse{
		AccessKeyId:     awsCredentials.AccessKeyID,
		SecretAccessKey: awsCredentials.SecretAccessKey,
		SessionToken:    awsCredentials.SessionToken,
		Expiration:      timestamppb.New(awsCredentials.Expiration),
	}, nil
}

func (s *Service) getAWSRolesAnywhereIntegrationSpec(ctx context.Context, integrationName string) (*types.AWSRAIntegrationSpecV1, error) {
	integration, err := s.cache.GetIntegration(ctx, integrationName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	spec := integration.GetAWSRolesAnywhereIntegrationSpec()
	if spec == nil {
		return nil, trace.BadParameter("integration %q is not an AWSRA integration", integrationName)
	}

	return spec, nil
}

// AWSRolesAnywhereServiceConfig holds configuration options for the AWSRolesAnywhere Integration gRPC service.
type AWSRolesAnywhereServiceConfig struct {
	// IntegrationService is the service that provides access to integration credentials.
	IntegrationService *Service
	// Authorizer is the authorizer used to check access to the integration.
	Authorizer authz.Authorizer

	Clock  clockwork.Clock
	Logger *slog.Logger

	// newPingClient is used to initialize a PingClient.
	// If nil, the service will create a new PingClient using the AWS client config.
	// This is useful for testing purposes, allowing to inject a mock client.
	newPingClient func(ctx context.Context, req *awsra.AWSClientConfig) (awsra.PingClient, error)
}

// CheckAndSetDefaults checks the AWSRolesAnywhereServiceConfig fields and returns an error if a required param is not provided.
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

	if s.newPingClient == nil {
		s.newPingClient = awsra.NewPingClient
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

	// newPingClient is used to initialize a PingClient.
	// If nil, the service will create a new PingClient using the AWS client config.
	// This is useful for testing purposes, allowing to inject a mock client.
	newPingClient func(ctx context.Context, req *awsra.AWSClientConfig) (awsra.PingClient, error)
}

// NewAWSRolesAnywhereService returns a new AWSRolesAnywhereService.
func NewAWSRolesAnywhereService(cfg *AWSRolesAnywhereServiceConfig) (*AWSRolesAnywhereService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ret := &AWSRolesAnywhereService{
		integrationService: cfg.IntegrationService,
		logger:             cfg.Logger,
		authorizer:         cfg.Authorizer,
		clock:              cfg.Clock,
		newPingClient:      cfg.newPingClient,
	}

	return ret, nil
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

	// ListRolesAnywhereProfiles uses the ProfileSync config's Profile and Role.
	spec, err := s.integrationService.getAWSRolesAnywhereIntegrationSpec(ctx, req.GetIntegration())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	trustAnchor := spec.TrustAnchorARN

	credentials, err := s.integrationService.generateAWSRACredentialsWithoutAuthZ(ctx, &integrationpb.GenerateAWSRACredentialsRequest{
		ProfileArn:                    spec.ProfileSyncConfig.ProfileARN,
		RoleArn:                       spec.ProfileSyncConfig.RoleARN,
		ProfileAcceptsRoleSessionName: spec.ProfileSyncConfig.ProfileAcceptsRoleSessionName,
		SubjectName:                   authCtx.Identity.GetIdentity().Username,
	}, trustAnchor)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	trustAnchorARNParsed, err := arn.Parse(trustAnchor)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	trustAnchorRegion := trustAnchorARNParsed.Region

	listRolesAnywhereClient, err := awsra.NewListRolesAnywhereProfilesClient(ctx, &awsra.AWSClientConfig{
		Credentials: awsra.Credentials{
			AccessKeyID:     credentials.AccessKeyId,
			SecretAccessKey: credentials.SecretAccessKey,
			SessionToken:    credentials.SessionToken,
			Expiration:      credentials.Expiration.AsTime(),
			Version:         1,
		},
		Region: trustAnchorRegion,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	profileList, err := awsra.ListRolesAnywhereProfiles(ctx, listRolesAnywhereClient, awsra.ListRolesAnywhereProfilesRequest{
		NextToken:          req.GetNextPageToken(),
		ProfileNameFilters: req.GetProfileNameFilters(),
		IgnoredProfileARNs: []string{spec.ProfileSyncConfig.ProfileARN},
		PageSize:           int(req.GetPageSize()),
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
// If the integration is absent from the request, then it will use the trust anchor, profile and role from the request.
// This is useful for testing an integration that was not yet created.
//
// If the integration is present in the request, it will use the trust anchor and the profile, role from the integration's ProfileSync config.
//
// It returns the caller identity and the number of AWS Roles Anywhere Profiles that are active.
func (s *AWSRolesAnywhereService) AWSRolesAnywherePing(ctx context.Context, req *integrationpb.AWSRolesAnywherePingRequest) (*integrationpb.AWSRolesAnywherePingResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}

	var trustAnchorARN, profileARN, roleARN string

	switch req.Mode.(type) {
	case *integrationpb.AWSRolesAnywherePingRequest_Integration:
		spec, err := s.integrationService.getAWSRolesAnywhereIntegrationSpec(ctx, req.GetIntegration())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		trustAnchorARN = spec.TrustAnchorARN
		profileARN = spec.ProfileSyncConfig.ProfileARN
		roleARN = spec.ProfileSyncConfig.RoleARN

	case *integrationpb.AWSRolesAnywherePingRequest_Custom:
		trustAnchorARN = req.GetCustom().GetTrustAnchorArn()
		profileARN = req.GetCustom().GetProfileArn()
		roleARN = req.GetCustom().GetRoleArn()

	default:
		return nil, trace.BadParameter("either integration or custom request must be provided")
	}

	s.logger.DebugContext(ctx, "Testing AWS IAM Roles Anywhere integration",
		"integration", req.GetIntegration(),
		"trust_anchor", trustAnchorARN,
		"profile_arn", profileARN,
		"role_arn", roleARN,
	)

	credentialsRequest := &integrationpb.GenerateAWSRACredentialsRequest{
		ProfileArn:  profileARN,
		RoleArn:     roleARN,
		SubjectName: authCtx.Identity.GetIdentity().Username,
	}

	trustAnchorARNParsed, err := arn.Parse(trustAnchorARN)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	trustAnchorRegion := trustAnchorARNParsed.Region

	credentials, err := s.integrationService.generateAWSRACredentialsWithoutAuthZ(ctx, credentialsRequest, trustAnchorARN)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pingClient, err := s.newPingClient(ctx, &awsra.AWSClientConfig{
		Credentials: awsra.Credentials{
			AccessKeyID:     credentials.AccessKeyId,
			SecretAccessKey: credentials.SecretAccessKey,
			SessionToken:    credentials.SessionToken,
			Expiration:      credentials.Expiration.AsTime(),
			Version:         1,
		},
		Region: trustAnchorRegion,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ignoredProfiles := []string{profileARN}

	pingResp, err := awsra.Ping(ctx, pingClient, ignoredProfiles)
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
