// Copyright 2023 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package userloginstate

import (
	"context"

	"github.com/gravitational/trace"

	userloginstatev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userloginstate/v1"
	"github.com/gravitational/teleport/api/types/userloginstate"
	conv "github.com/gravitational/teleport/api/types/userloginstate/convert/v1"
)

// Client is a user login state client that conforms to the following lib/services interfaces:
// * services.UserLoginStates
type Client struct {
	grpcClient userloginstatev1.UserLoginStateServiceClient
}

// NewClient creates a new user login state client.
func NewClient(grpcClient userloginstatev1.UserLoginStateServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// GetUserLoginStates returns a list of all user login states.
// Deprecated: Prefer paginated variant such as ListUserLoginStates.
func (c *Client) GetUserLoginStates(ctx context.Context) ([]*userloginstate.UserLoginState, error) {
	resp, err := c.grpcClient.GetUserLoginStates(ctx, &userloginstatev1.GetUserLoginStatesRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ulsList := make([]*userloginstate.UserLoginState, len(resp.UserLoginStates))
	for i, uls := range resp.UserLoginStates {
		var err error
		ulsList[i], err = conv.FromProto(uls)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return ulsList, nil
}

// GetUserLoginState returns the specified user login state resource.
func (c *Client) GetUserLoginState(ctx context.Context, name string) (*userloginstate.UserLoginState, error) {
	resp, err := c.grpcClient.GetUserLoginState(ctx, &userloginstatev1.GetUserLoginStateRequest{
		Name: name,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uls, err := conv.FromProto(resp)
	return uls, trace.Wrap(err)
}

// UpsertUserLoginState creates or updates a user login state resource.
func (c *Client) UpsertUserLoginState(ctx context.Context, uls *userloginstate.UserLoginState) (*userloginstate.UserLoginState, error) {
	resp, err := c.grpcClient.UpsertUserLoginState(ctx, &userloginstatev1.UpsertUserLoginStateRequest{
		UserLoginState: conv.ToProto(uls),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	responseUls, err := conv.FromProto(resp)
	return responseUls, trace.Wrap(err)
}

// DeleteUserLoginState removes the specified user login state resource.
func (c *Client) DeleteUserLoginState(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeleteUserLoginState(ctx, &userloginstatev1.DeleteUserLoginStateRequest{
		Name: name,
	})
	return trace.Wrap(err)
}

// DeleteAllUserLoginStates removes all user login states.
func (c *Client) DeleteAllUserLoginStates(ctx context.Context) error {
	_, err := c.grpcClient.DeleteAllUserLoginStates(ctx, &userloginstatev1.DeleteAllUserLoginStatesRequest{})
	return trace.Wrap(err)
}

// ListUserLoginStates returns a paginated list of user login state resources.
func (c *Client) ListUserLoginStates(ctx context.Context, pageSize int, nextToken string) ([]*userloginstate.UserLoginState, string, error) {
	resp, err := c.grpcClient.ListUserLoginStates(ctx, &userloginstatev1.ListUserLoginStatesRequest{
		PageSize:  int32(pageSize),
		PageToken: nextToken,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	out := make([]*userloginstate.UserLoginState, 0, len(resp.UserLoginStates))
	for _, v := range resp.UserLoginStates {
		item, err := conv.FromProto(v)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		out = append(out, item)
	}
	return out, resp.GetNextPageToken(), nil
}
