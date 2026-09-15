// Copyright 2024 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scim

import (
	"context"
	"strconv"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
)

// RateLimitError is returned by SCIM client methods when the server signals a
// rate or concurrency limit. It wraps a [trace.LimitExceededError] and carries
// the retry-after delay in seconds extracted from the gRPC response trailer.
type RateLimitError struct {
	// RetryAfterSeconds is the value of the "retry-after" gRPC trailer, or 0 if
	// the trailer was absent.
	RetryAfterSeconds int64
	// Err is the underlying LimitExceeded trace error.
	Err error
}

func (e *RateLimitError) Error() string { return e.Err.Error() }
func (e *RateLimitError) Unwrap() error { return e.Err }

// NoContentTrailer is the gRPC response trailer key the server sets on
// PatchSCIMResource when it applied the patch without materializing a
// representation of the resource - HTTP 204 No Content instead of 200 OK with a body
// See: https://datatracker.ietf.org/doc/html/rfc7644#section-3.5.2
// > On successful completion, the server either MUST return a 200 OK
// > response code and the entire resource within the response body,
// > subject to the "attributes" query parameter (see Section 3.9), or MAY
// > return HTTP status code 204 (No Content) and the appropriate response
// > headers for a successful PATCH request.  The server MUST return a 200
// > OK if the "attributes" parameter is specified in the request.
const NoContentTrailer = "x-scim-no-content"

// SupportsNoContentHeader is a gRPC request header PatchSCIMResourceV2 sets
// to declare that the caller understands NoContentTrailer.
// The SCIM auth server handler must only set NoContentTrailer
// when it sees this header on the request -
// otherwise a rolling upgrade where auth picks up the fast path before the
// proxy does (or vice versa) could lead to inconsistent behavior.
const SupportsNoContentHeader = "x-scim-supports-no-content"

// hasNoContentTrailer reports whether the server flagged the response via
// [NoContentTrailer].
func hasNoContentTrailer(trailer metadata.MD) bool {
	return len(trailer.Get(NoContentTrailer)) > 0
}

// ClientSupportsNoContent reports whether the incoming request declared
// support for NoContentTrailer via [SupportsNoContentHeader]. Server-side
// handlers must check this before setting NoContentTrailer.
func ClientSupportsNoContent(ctx context.Context) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	return len(md.Get(SupportsNoContentHeader)) > 0
}

// wrapRateLimitErr converts a gRPC error into a [*RateLimitError] when the
// error is a limit-exceeded error, extracting the retry-after value from the
// supplied trailer.
func wrapRateLimitErr(trailer metadata.MD, err error) error {
	if err == nil || !trace.IsLimitExceeded(err) {
		return err
	}
	rlErr := &RateLimitError{Err: err}
	if vals := trailer.Get("retry-after"); len(vals) > 0 {
		if n, parseErr := strconv.ParseInt(vals[0], 10, 64); parseErr == nil {
			rlErr.RetryAfterSeconds = n
		}
	}
	return rlErr
}

// Client wraps the underlying GRPC client with some more human-friendly tooling
type Client struct {
	grpcClient scimpb.SCIMServiceClient
}

func NewClientFromConn(cc grpc.ClientConnInterface) *Client {
	return NewClient(scimpb.NewSCIMServiceClient(cc))
}

func NewClient(grpcClient scimpb.SCIMServiceClient) *Client {
	return &Client{grpcClient: grpcClient}
}

// ListSCIMResources fetches resources of a given type.
func (c *Client) ListSCIMResources(ctx context.Context, req *scimpb.ListSCIMResourcesRequest) (*scimpb.ResourceList, error) {
	var trailer metadata.MD
	resp, err := c.grpcClient.ListSCIMResources(ctx, req, grpc.Trailer(&trailer))
	if err != nil {
		return nil, trace.Wrap(wrapRateLimitErr(trailer, err), "handling SCIM list request")
	}
	return resp, nil
}

// GetSCIMResource fetches a single SCIM resource from the server by name
func (c *Client) GetSCIMResource(ctx context.Context, req *scimpb.GetSCIMResourceRequest) (*scimpb.Resource, error) {
	var trailer metadata.MD
	resp, err := c.grpcClient.GetSCIMResource(ctx, req, grpc.Trailer(&trailer))
	if err != nil {
		return nil, trace.Wrap(wrapRateLimitErr(trailer, err), "handling SCIM get request")
	}
	return resp, nil
}

// CreateSCIMResource creates a new SCIM resource based on a supplied
// resource description
func (c *Client) CreateSCIMResource(ctx context.Context, req *scimpb.CreateSCIMResourceRequest) (*scimpb.Resource, error) {
	var trailer metadata.MD
	resp, err := c.grpcClient.CreateSCIMResource(ctx, req, grpc.Trailer(&trailer))
	if err != nil {
		return nil, trace.Wrap(wrapRateLimitErr(trailer, err), "handling SCIM create request")
	}
	return resp, nil
}

// UpdateSCIMResource handles a request to update a resource, returning a
// representation of the updated resource
func (c *Client) UpdateSCIMResource(ctx context.Context, req *scimpb.UpdateSCIMResourceRequest) (*scimpb.Resource, error) {
	var trailer metadata.MD
	res, err := c.grpcClient.UpdateSCIMResource(ctx, req, grpc.Trailer(&trailer))
	if err != nil {
		return nil, trace.Wrap(wrapRateLimitErr(trailer, err), "handling SCIM update request")
	}
	return res, nil
}

// DeleteSCIMResource handles a request to delete a resource.
func (c *Client) DeleteSCIMResource(ctx context.Context, req *scimpb.DeleteSCIMResourceRequest) (*emptypb.Empty, error) {
	var trailer metadata.MD
	res, err := c.grpcClient.DeleteSCIMResource(ctx, req, grpc.Trailer(&trailer))
	if err != nil {
		return nil, trace.Wrap(wrapRateLimitErr(trailer, err), "handling SCIM delete request")
	}
	return res, nil
}

// PatchSCIMResourceResponse is the result of a PatchSCIMResourceV2 call.
type PatchSCIMResourceResponse struct {
	// Resource is the patched resource, as reported by the server.
	Resource *scimpb.Resource
	// NoContent reports whether the server applied the patch without
	// materializing a representation of it - see [NoContentTrailer].
	// Callers exposing this over HTTP should treat that as a 204 No
	// Content instead of a 200 with Resource as the body.
	// See: https://datatracker.ietf.org/doc/html/rfc7644#section-3.5.2
	// > On successful completion, the server either MUST return a 200 OK
	// > response code and the entire resource within the response body,
	// > subject to the "attributes" query parameter (see Section 3.9), or MAY
	// > return HTTP status code 204 (No Content) and the appropriate response
	// > headers for a successful PATCH request.  The server MUST return a 200
	// > OK if the "attributes" parameter is specified in the request.
	NoContent bool
}

// PatchSCIMResource handles a request to patch a resource.
func (c *Client) PatchSCIMResource(ctx context.Context, request *scimpb.PatchSCIMResourceRequest) (*scimpb.Resource, error) {
	var trailer metadata.MD
	resp, err := c.grpcClient.PatchSCIMResource(ctx, request, grpc.Trailer(&trailer))
	if err != nil {
		return nil, trace.Wrap(wrapRateLimitErr(trailer, err), "handling SCIM patch request")
	}
	return resp, nil
}

// PatchSCIMResourceV2 behaves like PatchSCIMResource, but also reports
// whether the server applied the patch without materializing a
// representation of it, instead of collapsing that into a nil Resource.
func (c *Client) PatchSCIMResourceV2(ctx context.Context, request *scimpb.PatchSCIMResourceRequest) (*PatchSCIMResourceResponse, error) {
	// Declare support for NoContentTrailer so the server knows it's safe
	// to set it - see SupportsNoContentHeader.
	ctx = metadata.AppendToOutgoingContext(ctx, SupportsNoContentHeader, "true")
	var trailer metadata.MD
	resp, err := c.grpcClient.PatchSCIMResource(ctx, request, grpc.Trailer(&trailer))
	if err != nil {
		return nil, trace.Wrap(wrapRateLimitErr(trailer, err), "handling SCIM patch request")
	}
	return &PatchSCIMResourceResponse{Resource: resp, NoContent: hasNoContentTrailer(trailer)}, nil
}
