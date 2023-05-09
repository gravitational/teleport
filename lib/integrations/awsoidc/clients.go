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

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
)

// RDSClientRequest contains the required fields to generate an Authenticated [rds.Client].
type RDSClientRequest struct {
	// Token is the token used to issue the API Call.
	Token string

	// RoleARN is the IAM Role ARN to assume.
	RoleARN string

	// Region where the API call should be made.
	Region string
}

// CheckAndSetDefaults checks if the required fields are present.
func (req *RDSClientRequest) CheckAndSetDefaults() error {
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

// NewRDSClient creates an [rds.Client] using the provided Token, RoleARN, Region and, optionally, a custom HTTP Client.
func NewRDSClient(ctx context.Context, req RDSClientRequest) (*rds.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(req.Region))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.Credentials = stscreds.NewWebIdentityRoleProvider(
		sts.NewFromConfig(cfg),
		req.RoleARN,
		IdentityToken(req.Token),
	)

	return rds.NewFromConfig(cfg), nil
}

// IdentityToken is an implementation of [stscreds.IdentityTokenRetriever] for returning a static token.
type IdentityToken string

// GetIdentityToken returns the token configured.
func (j IdentityToken) GetIdentityToken() ([]byte, error) {
	return []byte(j), nil
}
