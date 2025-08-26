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

package auth

import (
	"context"
	"errors"
	"time"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/integrations/awsra"
)

var errAppWithoutAWSClientSideCredentials = errors.New("target resource is not an application that sends credentials to the client")

// generateAWSClientSideCredentials generates AWS Credentials for the given application.
// If the target resource is not an application with an associated AWS Roles Anywhere integration,
// it returns errAppWithoutAWSClientSideCredentials error, which should be handled gracefully by the caller.
//
// The AWS Credentials returned are in the format expected by the AWS CLI/SDK config file `credential_process` (usually located at ~/.aws/config).
// Expected format documentation: https://docs.aws.amazon.com/sdkref/latest/guide/feature-process-credentials.html
func generateAWSClientSideCredentials(
	ctx context.Context,
	a *Server,
	req certRequest,
	notAfter time.Time,
) (string, error) {
	if req.appName == "" || req.awsRoleARN == "" {
		return "", errAppWithoutAWSClientSideCredentials
	}

	appInfo, err := getAppServerByName(ctx, a, req.appName)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Only integrations can generate credentials.
	integrationName := appInfo.GetIntegration()
	if integrationName == "" {
		return "", errAppWithoutAWSClientSideCredentials
	}

	integration, err := a.GetIntegration(ctx, integrationName)
	if err != nil {
		return "", trace.Wrap(err)
	}

	switch integration.GetSubKind() {
	case types.IntegrationSubKindAWSRolesAnywhere:
		// Only AWS Roles Anywhere integrations can generate credentials.
		return generateAWSRolesAnywhereCredentials(ctx, a, req, appInfo, integration, notAfter)

	case types.IntegrationSubKindAWSOIDC:
		// AWS access using AWS OIDC integration will proxy requests, but will not send any credentials to the client.
		return "", errAppWithoutAWSClientSideCredentials

	default:
		return "", trace.BadParameter("application %q is using integration %q for access, which does not support AWS credential generation", req.appName, integrationName)
	}
}

func getAppServerByName(ctx context.Context, a *Server, appServerName string) (types.Application, error) {
	appServers, err := a.GetApplicationServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, s := range appServers {
		if s.GetName() == appServerName {
			return s.GetApp(), nil
		}
	}
	return nil, trace.NotFound("application %q not found", appServerName)
}

func generateAWSRolesAnywhereCredentials(
	ctx context.Context,
	a *Server,
	req certRequest,
	appInfo types.Application,
	integration types.Integration,
	notAfter time.Time,
) (string, error) {
	integrationSpec := integration.GetAWSRolesAnywhereIntegrationSpec()
	if integrationSpec == nil || integrationSpec.TrustAnchorARN == "" {
		return "", trace.BadParameter("roles anywhere integration %q does not have a valid spec", integration.GetName())
	}

	awsProfileARN := appInfo.GetAWSRolesAnywhereProfileARN()
	acceptRoleSessionName := appInfo.GetAWSRolesAnywhereAcceptRoleSessionName()
	if awsProfileARN == "" {
		return "", trace.BadParameter("application %q does not have a valid AWS Roles Anywhere Profile ARN", req.appName)
	}

	durationSeconds := int(notAfter.Sub(a.clock.Now()).Seconds())

	generateCredentialsRequest := awsra.GenerateCredentialsRequest{
		Clock:                 a.clock,
		TrustAnchorARN:        integrationSpec.TrustAnchorARN,
		ProfileARN:            awsProfileARN,
		RoleARN:               req.awsRoleARN,
		SubjectCommonName:     req.user.GetName(),
		KeyStoreManager:       a.keyStore,
		AcceptRoleSessionName: acceptRoleSessionName,
		DurationSeconds:       &durationSeconds,
		Cache:                 a.Cache,
	}

	// Replace by the override function if it is set.
	// This method does an HTTP call to AWS services, so this is useful for mocking that call.
	// Only used in tests.
	if a.AWSRolesAnywhereCreateSessionOverride != nil {
		generateCredentialsRequest.CreateSession = a.AWSRolesAnywhereCreateSessionOverride
	}

	resp, err := awsra.GenerateCredentials(ctx, generateCredentialsRequest)
	if err != nil {
		return "", trace.Wrap(err)
	}

	encodedCredentials, err := resp.EncodeCredentialProcessFormat()
	return encodedCredentials, trace.Wrap(err)
}
