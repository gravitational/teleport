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
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	smithyendpoints "github.com/aws/smithy-go/endpoints"
	"github.com/gravitational/trace"

	cloudaws "github.com/gravitational/teleport/lib/cloud/imds/aws"
)

const (
	// AWS SignedHeaders will always be lowercase
	// https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-auth-using-authorization-header.html#sigv4-auth-header-overview
	challengeHeaderKey = "x-teleport-challenge"
)

type stsIdentityRequestOptions struct {
	fipsEndpointOption aws.FIPSEndpointState
	imdsClient         imdsClient
}

type stsIdentityRequestOption func(cfg *stsIdentityRequestOptions)

// WithFIPSEndpoint is a functional option to use a FIPS STS endpoint. In non-US
// regions, this will use the us-east-1 FIPS endpoint.
func WithFIPSEndpoint(useFIPS bool) stsIdentityRequestOption {
	return func(opts *stsIdentityRequestOptions) {
		if useFIPS {
			opts.fipsEndpointOption = aws.FIPSEndpointStateEnabled
		} else {
			opts.fipsEndpointOption = aws.FIPSEndpointStateUnset
		}
	}
}

// WithIMDSClient is a functional option to use a custom IMDS client.
func WithIMDSClient(clt imdsClient) stsIdentityRequestOption {
	return func(opts *stsIdentityRequestOptions) {
		opts.imdsClient = clt
	}
}

type imdsClient interface {
	// IsAvailable should return true if the IMDSv2 is available, and false
	// otherwise.
	IsAvailable(context.Context) bool
	// GetRegion should return the local region as reported by the IMDSv2.
	GetRegion(context.Context) (string, error)
}

// CreateSignedSTSIdentityRequest is called on the client side and returns an
// sts:GetCallerIdentity request signed with the local AWS credentials
func CreateSignedSTSIdentityRequest(ctx context.Context, challenge string, opts ...stsIdentityRequestOption) ([]byte, error) {
	var options stsIdentityRequestOptions
	for _, opt := range opts {
		opt(&options)
	}

	awsConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "loading default AWS config")
	}

	var signedRequest bytes.Buffer
	stsClient := sts.NewFromConfig(awsConfig,
		sts.WithEndpointResolverV2(newCustomResolver(challenge, &options)),
		func(stsOpts *sts.Options) {
			stsOpts.EndpointOptions.UseFIPSEndpoint = options.fipsEndpointOption
			// Use a fake HTTP client to record the request.
			stsOpts.HTTPClient = &httpRequestRecorder{&signedRequest}
			// httpRequestRecorder intentionally records the request and returns
			// an error, don't retry.
			stsOpts.RetryMaxAttempts = 1
		})

	if _, err = stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{}); !errors.Is(err, errRequestRecorded) {
		if err == nil {
			return nil, trace.Errorf("expected to get errRequestedRecorded, got <nil> (this is a bug)")
		}
		return nil, trace.Wrap(err, "building signed sts:GetCallerIdentity request")
	}

	return signedRequest.Bytes(), nil
}

// getEC2LocalRegion returns the AWS region this EC2 instance is running in, or
// a NotFound error if the EC2 IMDS is unavailable.
func getEC2LocalRegion(ctx context.Context, opts *stsIdentityRequestOptions) (string, error) {
	imdsClient := opts.imdsClient
	if imdsClient == nil {
		var err error
		imdsClient, err = cloudaws.NewInstanceMetadataClient(ctx)
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	if !imdsClient.IsAvailable(ctx) {
		return "", trace.NotFound("IMDS is unavailable")
	}

	region, err := imdsClient.GetRegion(ctx)
	return region, trace.Wrap(err)
}

type customResolver struct {
	defaultResolver sts.EndpointResolverV2
	challenge       string
	opts            *stsIdentityRequestOptions
}

func newCustomResolver(challenge string, opts *stsIdentityRequestOptions) *customResolver {
	return &customResolver{
		defaultResolver: sts.NewDefaultEndpointResolverV2(),
		challenge:       challenge,
		opts:            opts,
	}
}

// ResolveEndpoint implements [sts.EndpointResolverV2].
func (r customResolver) ResolveEndpoint(ctx context.Context, params sts.EndpointParameters) (smithyendpoints.Endpoint, error) {
	if aws.ToString(params.Region) == "" {
		// If we don't have a region from the environment here this will fail to
		// resolve. We can try to get the local region from IMDSv2 if running on EC2.
		region, err := getEC2LocalRegion(ctx, r.opts)
		switch {
		case trace.IsNotFound(err):
			params.Region = aws.String("aws-global")
			params.UseGlobalEndpoint = aws.Bool(true)
		case err != nil:
			return smithyendpoints.Endpoint{}, trace.Wrap(err, "failed to resolve local AWS region from environment or IMDS")
		default:
			params.Region = aws.String(region)
		}
	}
	endpoint, err := r.defaultResolver.ResolveEndpoint(ctx, params)
	if err != nil {
		return smithyendpoints.Endpoint{}, trace.Wrap(err)
	}
	if aws.ToBool(params.UseFIPS) && !slices.Contains(FIPSSTSEndpoints(), endpoint.URI.Host) {
		// The default resolver will return non-existent endpoints if FIPS was
		// requested in regions outside the USA. Use the FIPS endpoint in
		// us-east-1 instead.
		slog.InfoContext(ctx, "The AWS SDK resolved an invalid FIPS STS endpoint, attempting to use the us-east-1 FIPS STS endpoint instead. This will fail in non-default AWS partitions.", "resolved", endpoint.URI.Host)
		endpoint.URI.Host = fipsSTSEndpointUSEast1
	}
	// Add challenge as a header to be signed.
	endpoint.Headers.Add(challengeHeaderKey, r.challenge)
	// Request JSON for simpler parsing.
	endpoint.Headers.Add("Accept", "application/json")
	return endpoint, nil
}

type httpRequestRecorder struct {
	w io.Writer
}

var errRequestRecorded = errors.New("request recorded")

func (r *httpRequestRecorder) Do(req *http.Request) (*http.Response, error) {
	if err := req.Write(r.w); err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, errRequestRecorded
}
