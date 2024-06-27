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
package aws

import (
	"context"
	"errors"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go/aws/awserr"
	smithyendpoints "github.com/aws/smithy-go/endpoints"
	"github.com/gravitational/trace"
)

// STSAPI defines an interface for sts.Client.
type STSAPI interface {
	stscreds.AssumeRoleAPIClient

	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

type stsClient struct {
	client *sts.Client
}

// NewSTSClient returns a new STS client.
func NewSTSClient(config aws.Config) *stsClient {
	return &stsClient{
		client: sts.NewFromConfig(
			config,
			sts.WithEndpointResolverV2(newSTSEndpointResolver()),
		),
	}
}

func (c *stsClient) AssumeRole(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	output, err := c.client.AssumeRole(ctx, params, optFns...)
	if err != nil {
		return nil, trace.Wrap(convertSTSError(err))
	}
	return output, nil
}

func (c *stsClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	output, err := c.client.GetCallerIdentity(ctx, params, optFns...)
	if err != nil {
		return nil, trace.Wrap(convertSTSError(err))
	}
	return output, nil
}

// convertSTSError converts aws-sdk-go-v2/service/sts errors.
func convertSTSError(err error) error {
	if err == nil {
		return nil
	}

	// TODO(greedy52) is there any way to prevent aws-sdk-go-v2 to translate
	// HTTP error codes to these specific types? Is it possible to
	// automatically generate these so CI can catch new error types?
	var awsErr awserr.Error
	if errors.As(err, &awsErr) {
		switch awsErr.Code() {
		case "ExpiredTokenException":
			return trace.AccessDenied(awsErr.Error())
		case "IDPCommunicationError":
			return trace.ConnectionProblem(awsErr, awsErr.Error())
		case "IDPRejectedClaim":
			return trace.AccessDenied(awsErr.Error())
		case "InvalidAuthorizationMessageException":
			return trace.BadParameter(awsErr.Error())
		case "InvalidIdentityToken":
			return trace.AccessDenied(awsErr.Error())
		case "MalformedPolicyDocument":
			return trace.BadParameter(awsErr.Error())
		case "PackedPolicyTooLarge":
			return trace.BadParameter(awsErr.Error())
		case "RegionDisabledException":
			return trace.BadParameter(awsErr.Error())
		}
	}

	return convertGenericSDKV2error(err)
}

// stsEndpointResolver implements sts.EndpointResolverV2 to find the STS
// endpoint.
//
// Note that aws-sdk-go-v2 always uses regional endpoint by default. This
// custom endpoint resolver makes sure that we fall back to global endpoint if
// no region is available.
type stsEndpointResolver struct {
	defaultResolver sts.EndpointResolverV2
}

func newSTSEndpointResolver() stsEndpointResolver {
	return stsEndpointResolver{
		defaultResolver: sts.NewDefaultEndpointResolverV2(),
	}
}

// ResolveEndpoint implements sts.EndpointResolverV2 to find the STS endpoint.
func (r stsEndpointResolver) ResolveEndpoint(ctx context.Context, params sts.EndpointParameters) (smithyendpoints.Endpoint, error) {
	// Use the global endpoint when region is empty.
	if aws.ToString(params.Region) == "" {
		// A region is still required for passing basic validation and selecting
		// partition. Defaulting to "us-east-1".
		//
		// Note that for other partitions like US Gov, users are required to
		// specify region through env var like AWS_REGION. This was required
		// with aws-sdk-go as well otherwise the legacy SDK would fallback to
		// the global endpoint in the standard partition.
		params.Region = aws.String("us-east-1")
		params.UseGlobalEndpoint = aws.Bool(true)
		slog.DebugContext(ctx, "No region provided when resolving STS endpoint. Falling back to global endpoint.")
	}
	endpoint, err := r.defaultResolver.ResolveEndpoint(ctx, params)
	return endpoint, trace.Wrap(err)
}
