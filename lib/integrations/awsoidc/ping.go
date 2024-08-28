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

package awsoidc

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
)

// PingResponse contains the response for the integration.
type PingResponse struct {
	// The AWS account ID number of the account that owns or contains the calling entity.
	AccountID string
	// The AWS ARN associated with the calling entity.
	ARN string
	// The unique identifier of the calling entity.
	UserID string
}

// PingClient describes the required methods to list AWS VPCs.
type PingClient interface {
	// GetCallerIdentity does AWS STS GetCallerIdentity API request and returns its result.
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

type defaultPingClient struct {
	*sts.Client
}

// NewPingClient creates a new PingClient using an AWSClientRequest.
func NewPingClient(ctx context.Context, req *AWSClientRequest) (PingClient, error) {
	stsClient, err := newSTSClient(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultPingClient{
		Client: stsClient,
	}, nil
}

// Ping does a health check for an integration.
// Calls the following AWS API:
// https://docs.aws.amazon.com/STS/latest/APIReference/API_GetCallerIdentity.html
// It returns the current AWS IAM Identity.
func Ping(ctx context.Context, clt PingClient) (*PingResponse, error) {
	callerIndetityOut, err := clt.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &PingResponse{
		AccountID: aws.ToString(callerIndetityOut.Account),
		ARN:       aws.ToString(callerIndetityOut.Arn),
		UserID:    aws.ToString(callerIndetityOut.UserId),
	}, nil
}
