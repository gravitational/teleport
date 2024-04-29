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

package awsoidc

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	utilsaws "github.com/gravitational/teleport/api/utils/aws"
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

	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigDisable,
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

	awsConfig := aws.NewConfig().
		WithRegion(region).
		WithCredentials(awsCredentials)

	session, err := session.NewSessionWithOptions(session.Options{
		Config:            *awsConfig,
		SharedConfigState: session.SharedConfigDisable,
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
