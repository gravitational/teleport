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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
)

// AWSClientRequest contains the required fields to set up an AWS service client.
type AWSClientRequest struct {
	// Token is the token used to issue the API Call.
	Token string

	// RoleARN is the IAM Role ARN to assume.
	RoleARN string

	// Region where the API call should be made.
	Region string

	// httpClient used in tests.
	httpClient aws.HTTPClient
}

// CheckAndSetDefaults checks if the required fields are present.
func (req *AWSClientRequest) CheckAndSetDefaults() error {
	if req.Token == "" {
		return trace.BadParameter("token is required")
	}

	if req.RoleARN == "" {
		return trace.BadParameter("role arn is required")
	}

	if req.Region == "" {
		return trace.BadParameter("region is required")
	}

	return nil
}

// newAWSConfig creates a new [aws.Config] using the [AWSClientRequest] fields.
func newAWSConfig(ctx context.Context, req *AWSClientRequest) (*aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(req.Region))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.httpClient != nil {
		cfg.HTTPClient = req.httpClient
	}

	cfg.Credentials = stscreds.NewWebIdentityRoleProvider(
		sts.NewFromConfig(cfg),
		req.RoleARN,
		IdentityToken(req.Token),
	)

	return &cfg, nil
}

// newRDSClient creates an [rds.Client] using the provided Token, RoleARN and Region.
func newRDSClient(ctx context.Context, req *AWSClientRequest) (*rds.Client, error) {
	cfg, err := newAWSConfig(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rds.NewFromConfig(*cfg), nil
}

// newECSClient creates an [ecs.Client] using the provided Token, RoleARN and Region.
func newECSClient(ctx context.Context, req *AWSClientRequest) (*ecs.Client, error) {
	cfg, err := newAWSConfig(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ecs.NewFromConfig(*cfg), nil
}

// IdentityToken is an implementation of [stscreds.IdentityTokenRetriever] for returning a static token.
type IdentityToken string

// GetIdentityToken returns the token configured.
func (j IdentityToken) GetIdentityToken() ([]byte, error) {
	return []byte(j), nil
}
