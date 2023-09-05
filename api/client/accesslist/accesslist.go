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

package accesslist

import (
	"context"

	"github.com/gravitational/trace"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
)

// Client is an access list client that conforms to the following lib/services interfaces:
// * services.AccessLists
type Client struct {
	grpcClient accesslistv1.AccessListServiceClient
}

// NewClient creates a new Access List client.
func NewClient(grpcClient accesslistv1.AccessListServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// GetAccessLists returns a list of all access lists.
func (c *Client) GetAccessLists(ctx context.Context) ([]*accesslist.AccessList, error) {
	resp, err := c.grpcClient.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessLists := make([]*accesslist.AccessList, len(resp.AccessLists))
	for i, accessList := range resp.AccessLists {
		var err error
		accessLists[i], err = conv.FromProto(accessList)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return accessLists, nil
}

// ListAccessLists returns a paginated list of access lists.
func (c *Client) ListAccessLists(ctx context.Context, pageSize int, nextToken string) ([]*accesslist.AccessList, string, error) {
	resp, err := c.grpcClient.ListAccessLists(ctx, &accesslistv1.ListAccessListsRequest{
		PageSize:  int32(pageSize),
		NextToken: nextToken,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	accessLists := make([]*accesslist.AccessList, len(resp.AccessLists))
	for i, accessList := range resp.AccessLists {
		var err error
		accessLists[i], err = conv.FromProto(accessList)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	return accessLists, resp.GetNextToken(), nil
}

// GetAccessList returns the specified access list resource.
func (c *Client) GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error) {
	resp, err := c.grpcClient.GetAccessList(ctx, &accesslistv1.GetAccessListRequest{
		Name: name,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessList, err := conv.FromProto(resp)
	return accessList, trace.Wrap(err)
}

// UpsertAccessList creates or updates an access list resource.
func (c *Client) UpsertAccessList(ctx context.Context, accessList *accesslist.AccessList) (*accesslist.AccessList, error) {
	resp, err := c.grpcClient.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{
		AccessList: conv.ToProto(accessList),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	responseAccessList, err := conv.FromProto(resp)
	return responseAccessList, trace.Wrap(err)
}

// DeleteAccessList removes the specified access list resource.
func (c *Client) DeleteAccessList(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeleteAccessList(ctx, &accesslistv1.DeleteAccessListRequest{
		Name: name,
	})
	return trace.Wrap(err)
}

// DeleteAllAccessLists removes all access lists.
func (c *Client) DeleteAllAccessLists(ctx context.Context) error {
	return trace.NotImplemented("DeleteAllAccessLists not supported in the gRPC client")
}

// ListAccessListMembers returns a paginated list of all access list members for an access list.
func (c *Client) ListAccessListMembers(ctx context.Context, accessList string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error) {
	resp, err := c.grpcClient.ListAccessListMembers(ctx, &accesslistv1.ListAccessListMembersRequest{
		PageSize:   int32(pageSize),
		PageToken:  pageToken,
		AccessList: accessList,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	members = make([]*accesslist.AccessListMember, len(resp.Members))
	for i, accessList := range resp.Members {
		var err error
		members[i], err = conv.FromMemberProto(accessList)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	return members, resp.GetNextPageToken(), nil
}

// GetAccessListMember returns the specified access list member resource.
func (c *Client) GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error) {
	resp, err := c.grpcClient.GetAccessListMember(ctx, &accesslistv1.GetAccessListMemberRequest{
		AccessList: accessList,
		MemberName: memberName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	member, err := conv.FromMemberProto(resp)
	return member, trace.Wrap(err)
}

// UpsertAccessListMember creates or updates an access list member resource.
func (c *Client) UpsertAccessListMember(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error) {
	resp, err := c.grpcClient.UpsertAccessListMember(ctx, &accesslistv1.UpsertAccessListMemberRequest{
		Member: conv.ToMemberProto(member),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	responseMember, err := conv.FromMemberProto(resp)
	return responseMember, trace.Wrap(err)
}

// DeleteAccessListMember hard deletes the specified access list member resource.
func (c *Client) DeleteAccessListMember(ctx context.Context, accessList string, memberName string) error {
	_, err := c.grpcClient.DeleteAccessListMember(ctx, &accesslistv1.DeleteAccessListMemberRequest{
		AccessList: accessList,
		MemberName: memberName,
	})
	return trace.Wrap(err)
}

// DeleteAllAccessListMembersForAccessList hard deletes all access list members for an access list.
func (c *Client) DeleteAllAccessListMembersForAccessList(ctx context.Context, accessList string) error {
	_, err := c.grpcClient.DeleteAllAccessListMembersForAccessList(ctx, &accesslistv1.DeleteAllAccessListMembersForAccessListRequest{
		AccessList: accessList,
	})
	return trace.Wrap(err)
}

// DeleteAllAccessListMembers hard deletes all access list members.
func (c *Client) DeleteAllAccessListMembers(ctx context.Context) error {
	return trace.NotImplemented("DeleteAllAccessListMembers is not supported in the gRPC client")
}

// UpsertAccessListWithMembers creates or updates an access list resource and its members.
func (c *Client) UpsertAccessListWithMembers(ctx context.Context, list *accesslist.AccessList, members []*accesslist.AccessListMember) (*accesslist.AccessList, []*accesslist.AccessListMember, error) {
	resp, err := c.grpcClient.UpsertAccessListWithMembers(ctx, &accesslistv1.UpsertAccessListWithMembersRequest{
		AccessList: conv.ToProto(list),
		Members:    conv.ToMembersProto(members),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	accessList, err := conv.FromProto(resp.AccessList)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	updatedMembers, err := conv.FromMembersProto(resp.Members)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return accessList, updatedMembers, nil
}
