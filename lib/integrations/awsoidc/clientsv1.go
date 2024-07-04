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

package awsoidc

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	utilsaws "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/modules"
)

// FetchToken returns the token.
func (j IdentityToken) FetchToken(ctx credentials.Context) ([]byte, error) {
	return []byte(j), nil
}

// IntegrationTokenGenerator is an interface that indicates which APIs are required to generate an Integration Token.
type IntegrationTokenGenerator interface {
	// GetIntegration returns the specified integration resources.
	GetIntegration(ctx context.Context, name string) (types.Integration, error)

	// GetProxies returns a list of registered proxies.
	GetProxies() ([]types.Server, error)

	// GenerateAWSOIDCToken generates a token to be used to execute an AWS OIDC Integration action.
	GenerateAWSOIDCToken(ctx context.Context, integration string) (string, error)
}

// NewSessionV1 creates a new AWS Session for the region using the integration as source of credentials.
// This session is usable for AWS SDK Go V1.
func NewSessionV1(ctx context.Context, client IntegrationTokenGenerator, region string, integrationName string) (*session.Session, error) {
	if region != "" {
		if err := utilsaws.IsValidRegion(region); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	integration, err := client.GetIntegration(ctx, integrationName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	awsOIDCIntegration := integration.GetAWSOIDCIntegrationSpec()
	if awsOIDCIntegration == nil {
		return nil, trace.BadParameter("invalid integration subkind, expected awsoidc, got %s", integration.GetSubKind())
	}

	useFIPSEndpoint := endpoints.FIPSEndpointStateUnset
	if modules.GetModules().IsBoringBinary() {
		useFIPSEndpoint = endpoints.FIPSEndpointStateEnabled
	}

	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigDisable,
		Config: aws.Config{
			UseFIPSEndpoint: useFIPSEndpoint,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// AWS SDK calls FetchToken everytime the session is no longer valid (or there's no session).
	// Generating a token here and using it as a Static would make this token valid for the Max Duration Session for the current AWS Role (usually, 1 hour).
	// Instead, it generates a token everytime the Session's client requests a new token, ensuring it always receives a fresh one.
	var integrationTokenFetcher IntegrationTokenFetcher = func(ctx context.Context) ([]byte, error) {
		token, err := client.GenerateAWSOIDCToken(ctx, integrationName)
		return []byte(token), trace.Wrap(err)
	}

	stsSTS := sts.New(sess)
	roleProvider := stscreds.NewWebIdentityRoleProviderWithOptions(
		stsSTS,
		awsOIDCIntegration.RoleARN,
		"",
		integrationTokenFetcher,
	)
	awsCredentials := credentials.NewCredentials(roleProvider)

	session, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigDisable,
		Config: aws.Config{
			Region:          aws.String(region),
			Credentials:     awsCredentials,
			UseFIPSEndpoint: useFIPSEndpoint,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// IntegrationTokenFetcher handles dynamic token generation using a callback function.
// Useful to embed as a [stscreds.TokenFetcher].
type IntegrationTokenFetcher func(context.Context) ([]byte, error)

// FetchToken returns a token by calling the callback function.
func (genFn IntegrationTokenFetcher) FetchToken(ctx context.Context) ([]byte, error) {
	token, err := genFn(ctx)
	return token, trace.Wrap(err)
}
