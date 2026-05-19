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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"

	awsutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
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

	if req.Region != "" {
		if err := awsutils.IsValidRegion(req.Region); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// newAWSConfig creates a new [aws.Config] using the [AWSClientRequest] fields.
func newAWSConfig(ctx context.Context, req *AWSClientRequest) (*aws.Config, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(req.Region))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.httpClient != nil {
		cfg.HTTPClient = req.httpClient
	}

	cfg.Credentials = stscreds.NewWebIdentityRoleProvider(
		stsutils.NewFromConfig(cfg),
		req.RoleARN,
		IdentityToken(req.Token),
	)

	return &cfg, nil
}

func newEKSClient(ctx context.Context, req *AWSClientRequest) (*eks.Client, error) {
	cfg, err := newAWSConfig(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return eks.NewFromConfig(*cfg), nil
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

// newSTSClient creates an [sts.Client] using the provided Token, RoleARN and Region.
func newSTSClient(ctx context.Context, req *AWSClientRequest) (*sts.Client, error) {
	cfg, err := newAWSConfig(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return stsutils.NewFromConfig(*cfg), nil
}

// newEC2Client creates an [ec2.Client] using the provided Token, RoleARN and Region.
func newEC2Client(ctx context.Context, req *AWSClientRequest) (*ec2.Client, error) {
	cfg, err := newAWSConfig(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ec2.NewFromConfig(*cfg), nil
}

// newEC2InstanceConnectClient creates an [ec2instanceconnect.Client] using the provided Token, RoleARN and Region.
func newEC2InstanceConnectClient(ctx context.Context, req *AWSClientRequest) (*ec2instanceconnect.Client, error) {
	cfg, err := newAWSConfig(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ec2instanceconnect.NewFromConfig(*cfg), nil
}

// NewAWSCredentialsProvider creates an [aws.CredentialsProvider] using the provided Token, RoleARN and Region.
func NewAWSCredentialsProvider(ctx context.Context, req *AWSClientRequest) (aws.CredentialsProvider, error) {
	cfg, err := newAWSConfig(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cfg.Credentials, nil
}

// IdentityToken is an implementation of [stscreds.IdentityTokenRetriever] for returning a static token.
type IdentityToken string

// GetIdentityToken returns the token configured.
func (j IdentityToken) GetIdentityToken() ([]byte, error) {
	return []byte(j), nil
}

// CallerIdentityGetter is a subset of [sts.Client] that can be used to information about the caller identity.
type CallerIdentityGetter interface {
	// GetCallerIdentity returns information about the caller identity.
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// CheckAccountID is a helper func that check if the current caller account ID
// matches the expected account ID.
func CheckAccountID(ctx context.Context, clt CallerIdentityGetter, wantAccountID string) error {
	if wantAccountID == "" {
		return nil
	}
	callerIdentity, err := clt.GetCallerIdentity(ctx, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	currentAccountID := aws.ToString(callerIdentity.Account)
	if wantAccountID != currentAccountID {
		return trace.BadParameter("expected account ID %s but current account ID is %s", wantAccountID, currentAccountID)
	}
	return nil
}
