/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package mocks

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
)

type AWSConfigProvider struct {
	Err                   error
	AWSConfig             *aws.Config
	STSClient             *STSClient
	OIDCIntegrationClient awsconfig.OIDCIntegrationClient
}

func (f *AWSConfigProvider) GetConfig(ctx context.Context, region string, optFns ...awsconfig.OptionsFn) (aws.Config, error) {
	if f.Err != nil {
		return aws.Config{}, f.Err
	}

	if f.AWSConfig != nil {
		return *f.AWSConfig, nil
	}

	stsClt := f.STSClient
	if stsClt == nil {
		stsClt = &STSClient{}
	}

	// Build a minimal aws.Config without loading the developer's local shared config (~/.aws/config).
	// Apply the assume-role chain manually using the mock STS client so tests can verify which roles were assumed.
	cfg := aws.Config{Region: region}
	stsClientProvider := NewAssumeRoleClientProviderFunc(stsClt)
	roles := awsconfig.AssumedRoles(optFns...)
	for _, r := range roles {
		cfg.Credentials = stscreds.NewAssumeRoleProvider(stsClientProvider(cfg), r.RoleARN, func(aro *stscreds.AssumeRoleOptions) {
			if r.ExternalID != "" {
				aro.ExternalID = aws.String(r.ExternalID)
			}
			if r.SessionName != "" {
				aro.RoleSessionName = r.SessionName
			}
			aro.Duration = r.Duration
			for k, v := range r.Tags {
				aro.Tags = append(aro.Tags, ststypes.Tag{
					Key:   aws.String(k),
					Value: aws.String(v),
				})
			}
		})
	}
	if len(roles) > 0 {
		cfg.Credentials = aws.NewCredentialsCache(cfg.Credentials)
		if _, err := cfg.Credentials.Retrieve(ctx); err != nil {
			return aws.Config{}, trace.Wrap(err)
		}
	}
	return cfg, nil
}

type FakeOIDCIntegrationClient struct {
	Unauth bool

	Integration types.Integration
	Token       string
}

func (f *FakeOIDCIntegrationClient) GetIntegration(ctx context.Context, name string) (types.Integration, error) {
	if f.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	if f.Integration.GetName() == name {
		return f.Integration, nil
	}
	return nil, trace.NotFound("integration %q not found", name)
}

func (f *FakeOIDCIntegrationClient) GenerateAWSOIDCToken(ctx context.Context, integrationName string) (string, error) {
	if f.Unauth {
		return "", trace.AccessDenied("unauthorized")
	}
	return f.Token, nil
}
