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

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

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
			return s.generateAWSRACredentialsWithoutAuthZ(ctx, req)
		}
	}

	return nil, trace.AccessDenied("credential generation is only available to auth or proxy services")
}

// generateAWSRACredentialsWithoutAuthZ generates a set of AWS credentials which uses the AWS Roles Anywhere integration.
// Bypasses authz and should only be used by other methods that validate AuthZ.
func (s *Service) generateAWSRACredentialsWithoutAuthZ(ctx context.Context, req *integrationpb.GenerateAWSRACredentialsRequest) (*integrationpb.GenerateAWSRACredentialsResponse, error) {
	integration, err := s.cache.GetIntegration(ctx, req.GetIntegration())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	spec := integration.GetAWSRolesAnywhereIntegrationSpec()
	if spec == nil {
		return nil, trace.BadParameter("integration %q is not an AWSRA integration", req.Integration)
	}

	var durationSeconds *int
	if req.GetSessionMaxDuration().AsDuration() != 0 {
		d := int(req.GetSessionMaxDuration().AsDuration().Seconds())
		durationSeconds = &d
	}

	awsCredentials, err := awsra.GenerateCredentials(ctx, awsra.GenerateCredentialsRequest{
		Clock:                 s.clock,
		TrustAnchorARN:        spec.TrustAnchorARN,
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
