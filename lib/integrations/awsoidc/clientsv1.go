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
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils/oidc"
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
	GenerateAWSOIDCToken(ctx context.Context, req types.GenerateAWSOIDCTokenRequest) (string, error)
}

// NewSessionV1 creates a new AWS Session for the region using the integration as source of credentials.
// This session is usable for AWS SDK Go V1.
func NewSessionV1(ctx context.Context, client IntegrationTokenGenerator, region string, integrationName string) (*session.Session, error) {
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
			EC2MetadataEnableFallback: aws.Bool(false),
			UseFIPSEndpoint:           useFIPSEndpoint,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	issuer, err := oidc.IssuerForCluster(ctx, client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := client.GenerateAWSOIDCToken(ctx, types.GenerateAWSOIDCTokenRequest{
		Issuer: issuer,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stsSTS := sts.New(sess)
	roleProvider := stscreds.NewWebIdentityRoleProviderWithOptions(
		stsSTS,
		awsOIDCIntegration.RoleARN,
		"",
		IdentityToken(token),
	)
	awsCredentials := credentials.NewCredentials(roleProvider)

	session, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigDisable,
		Config: aws.Config{
			Region:                    aws.String(region),
			Credentials:               awsCredentials,
			EC2MetadataEnableFallback: aws.Bool(false),
			UseFIPSEndpoint:           useFIPSEndpoint,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}
