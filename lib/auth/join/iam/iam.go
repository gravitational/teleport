// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package iam

import (
	"bytes"
	"context"
	"log/slog"
	"slices"
	"strings"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/gravitational/trace"

	cloudaws "github.com/gravitational/teleport/lib/cloud/imds/aws"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

const (
	// AWS SignedHeaders will always be lowercase
	// https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-auth-using-authorization-header.html#sigv4-auth-header-overview
	challengeHeaderKey = "x-teleport-challenge"
)

type stsIdentityRequestConfig struct {
	regionalEndpointOption endpoints.STSRegionalEndpoint
	fipsEndpointOption     endpoints.FIPSEndpointState
}

type stsIdentityRequestOption func(cfg *stsIdentityRequestConfig)

func WithRegionalEndpoint(useRegionalEndpoint bool) stsIdentityRequestOption {
	return func(cfg *stsIdentityRequestConfig) {
		if useRegionalEndpoint {
			cfg.regionalEndpointOption = endpoints.RegionalSTSEndpoint
		} else {
			cfg.regionalEndpointOption = endpoints.LegacySTSEndpoint
		}
	}
}

func WithFIPSEndpoint(useFIPS bool) stsIdentityRequestOption {
	return func(cfg *stsIdentityRequestConfig) {
		if useFIPS {
			cfg.fipsEndpointOption = endpoints.FIPSEndpointStateEnabled
		} else {
			cfg.fipsEndpointOption = endpoints.FIPSEndpointStateDisabled
		}
	}
}

// getEC2LocalRegion returns the AWS region this EC2 instance is running in, or
// a NotFound error if the EC2 IMDS is unavailable.
func getEC2LocalRegion(ctx context.Context) (string, error) {
	imdsClient, err := cloudaws.NewInstanceMetadataClient(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if !imdsClient.IsAvailable(ctx) {
		return "", trace.NotFound("IMDS is unavailable")
	}

	region, err := imdsClient.GetRegion(ctx)
	return region, trace.Wrap(err)
}

func newSTSClient(ctx context.Context, cfg *stsIdentityRequestConfig) (*sts.STS, error) {
	awsConfig := awssdk.Config{
		UseFIPSEndpoint:     cfg.fipsEndpointOption,
		STSRegionalEndpoint: cfg.regionalEndpointOption,
	}
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config:            awsConfig,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stsClient := stsutils.NewV1(sess)

	if slices.Contains(GlobalSTSEndpoints(), strings.TrimPrefix(stsClient.Endpoint, "https://")) {
		// If the caller wants to use the regional endpoint but it was not resolved
		// from the environment, attempt to find the region from the EC2 IMDS
		if cfg.regionalEndpointOption == endpoints.RegionalSTSEndpoint {
			region, err := getEC2LocalRegion(ctx)
			if err != nil {
				return nil, trace.Wrap(err, "failed to resolve local AWS region from environment or IMDS")
			}
			stsClient = stsutils.NewV1(sess, awssdk.NewConfig().WithRegion(region))
		} else {
			const msg = "Attempting to use the global STS endpoint for the IAM join method. " +
				"This will probably fail in non-default AWS partitions such as China or GovCloud, or if FIPS mode is enabled. " +
				"Consider setting the AWS_REGION environment variable, setting the region in ~/.aws/config, or enabling the IMDSv2."
			slog.InfoContext(ctx, msg)
		}
	}

	if cfg.fipsEndpointOption == endpoints.FIPSEndpointStateEnabled &&
		!slices.Contains(ValidSTSEndpoints(), strings.TrimPrefix(stsClient.Endpoint, "https://")) {
		// The AWS SDK will generate invalid endpoints when attempting to
		// resolve the FIPS endpoint for a region that does not have one.
		// In this case, try to use the FIPS endpoint in us-east-1. This should
		// work for all regions in the standard partition. In GovCloud, we should
		// not hit this because all regional endpoints support FIPS. In China or
		// other partitions, this will fail, and FIPS mode will not be supported.
		const msg = "AWS SDK resolved invalid FIPS STS endpoint. " +
			"Attempting to use the FIPS STS endpoint for us-east-1."
		slog.InfoContext(ctx, msg, "resolved", stsClient.Endpoint)
		stsClient = stsutils.NewV1(sess, awssdk.NewConfig().WithRegion("us-east-1"))
	}

	return stsClient, nil
}

// CreateSignedSTSIdentityRequest is called on the client side and returns an
// sts:GetCallerIdentity request signed with the local AWS credentials
func CreateSignedSTSIdentityRequest(ctx context.Context, challenge string, opts ...stsIdentityRequestOption) ([]byte, error) {
	cfg := &stsIdentityRequestConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	stsClient, err := newSTSClient(ctx, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req, _ := stsClient.GetCallerIdentityRequest(&sts.GetCallerIdentityInput{})
	// set challenge header
	req.HTTPRequest.Header.Set(challengeHeaderKey, challenge)
	// request json for simpler parsing
	req.HTTPRequest.Header.Set("Accept", "application/json")
	// sign the request, including headers
	if err := req.Sign(); err != nil {
		return nil, trace.Wrap(err)
	}
	// write the signed HTTP request to a buffer
	var signedRequest bytes.Buffer
	if err := req.HTTPRequest.Write(&signedRequest); err != nil {
		return nil, trace.Wrap(err)
	}
	return signedRequest.Bytes(), nil
}
