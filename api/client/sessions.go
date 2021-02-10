/*
Copyright 2021 Gravitational, Inc.

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

package client

import (
	"context"

	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/trace/trail"

	"github.com/golang/protobuf/ptypes/empty"
)

// GetWebSession returns the web session for the specified request.
// Implements ReadAccessPoint
func (c *Client) GetWebSession(ctx context.Context, req types.GetWebSessionRequest) (types.WebSession, error) {
	return c.WebSessions().Get(ctx, req)
}

// WebSessions returns the web sessions controller
func (c *Client) WebSessions() types.WebSessionInterface {
	return &webSessions{c: c}
}

// Get returns the web session for the specified request
func (r *webSessions) Get(ctx context.Context, req types.GetWebSessionRequest) (types.WebSession, error) {
	resp, err := r.c.grpc.GetWebSession(ctx, &req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp.Session, nil
}

// List returns the list of all web sessions
func (r *webSessions) List(ctx context.Context) ([]types.WebSession, error) {
	resp, err := r.c.grpc.GetWebSessions(ctx, &empty.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	out := make([]types.WebSession, 0, len(resp.Sessions))
	for _, session := range resp.Sessions {
		out = append(out, session)
	}
	return out, nil
}

// Delete deletes the web session specified with the request
func (r *webSessions) Delete(ctx context.Context, req types.DeleteWebSessionRequest) error {
	_, err := r.c.grpc.DeleteWebSession(ctx, &req)
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// DeleteAll deletes all web sessions
func (r *webSessions) DeleteAll(ctx context.Context) error {
	_, err := r.c.grpc.DeleteAllWebSessions(ctx, &empty.Empty{})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

type webSessions struct {
	c *Client
}

// GetWebToken returns the web token for the specified request.
// Implements ReadAccessPoint
func (c *Client) GetWebToken(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error) {
	return c.WebTokens().Get(ctx, req)
}

// WebTokens returns the web tokens controller
func (c *Client) WebTokens() types.WebTokenInterface {
	return &webTokens{c: c}
}

// Get returns the web token for the specified request
func (r *webTokens) Get(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error) {
	resp, err := r.c.grpc.GetWebToken(ctx, &req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp.Token, nil
}

// List returns the list of all web tokens
func (r *webTokens) List(ctx context.Context) ([]types.WebToken, error) {
	resp, err := r.c.grpc.GetWebTokens(ctx, &empty.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	out := make([]types.WebToken, 0, len(resp.Tokens))
	for _, token := range resp.Tokens {
		out = append(out, token)
	}
	return out, nil
}

// Delete deletes the web token specified with the request
func (r *webTokens) Delete(ctx context.Context, req types.DeleteWebTokenRequest) error {
	_, err := r.c.grpc.DeleteWebToken(ctx, &req)
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// DeleteAll deletes all web tokens
func (r *webTokens) DeleteAll(ctx context.Context) error {
	_, err := r.c.grpc.DeleteAllWebTokens(ctx, &empty.Empty{})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

type webTokens struct {
	c *Client
}
