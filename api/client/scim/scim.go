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

// PatchSCIMResource handles a request to patch a resource.
func (c *Client) PatchSCIMResource(ctx context.Context, request *scimpb.PatchSCIMResourceRequest) (*scimpb.Resource, error) {
	var trailer metadata.MD
	resp, err := c.grpcClient.PatchSCIMResource(ctx, request, grpc.Trailer(&trailer))
	if err != nil {
		return nil, trace.Wrap(wrapRateLimitErr(trailer, err), "handling SCIM patch request")
	}
	return resp, nil
}
