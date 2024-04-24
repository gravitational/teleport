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
	"time"

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

	accessList, err := conv.FromProto(resp, conv.WithOwnersIneligibleStatusField(resp.GetSpec().GetOwners()))
	return accessList, trace.Wrap(err)
}

// GetAccessListsToReview returns access lists that the user needs to review.
func (c *Client) GetAccessListsToReview(ctx context.Context) ([]*accesslist.AccessList, error) {
	resp, err := c.grpcClient.GetAccessListsToReview(ctx, &accesslistv1.GetAccessListsToReviewRequest{})
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

// UpdateAccessList updates an access list resource.
func (c *Client) UpdateAccessList(ctx context.Context, accessList *accesslist.AccessList) (*accesslist.AccessList, error) {
	resp, err := c.grpcClient.UpdateAccessList(ctx, &accesslistv1.UpdateAccessListRequest{
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

// CountAccessListMembers will count all access list members.
func (c *Client) CountAccessListMembers(ctx context.Context, accessListName string) (uint32, error) {
	resp, err := c.grpcClient.CountAccessListMembers(ctx, &accesslistv1.CountAccessListMembersRequest{
		AccessListName: accessListName,
	})
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return resp.Count, nil
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
	for i, member := range resp.Members {
		var err error
		members[i], err = conv.FromMemberProto(member, conv.WithMemberIneligibleStatusField(member))
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	return members, resp.GetNextPageToken(), nil
}

// ListAllAccessListMembers returns a paginated list of all access list members for all access lists.
func (c *Client) ListAllAccessListMembers(ctx context.Context, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error) {
	resp, err := c.grpcClient.ListAllAccessListMembers(ctx, &accesslistv1.ListAllAccessListMembersRequest{
		PageSize:  int32(pageSize),
		PageToken: pageToken,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	members = make([]*accesslist.AccessListMember, len(resp.Members))
	for i, member := range resp.Members {
		var err error
		members[i], err = conv.FromMemberProto(member, conv.WithMemberIneligibleStatusField(member))
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

	member, err := conv.FromMemberProto(resp, conv.WithMemberIneligibleStatusField(resp))
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

// UpdateAccessListMember updates an access list member resource using a conditional update.
func (c *Client) UpdateAccessListMember(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error) {
	resp, err := c.grpcClient.UpdateAccessListMember(ctx, &accesslistv1.UpdateAccessListMemberRequest{
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

	accessList, err := conv.FromProto(resp.AccessList, conv.WithOwnersIneligibleStatusField(resp.AccessList.GetSpec().GetOwners()))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	updatedMembers := make([]*accesslist.AccessListMember, len(resp.Members))
	for i, member := range resp.Members {
		var err error
		updatedMembers[i], err = conv.FromMemberProto(member, conv.WithMemberIneligibleStatusField(member))
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	return accessList, updatedMembers, nil
}

// AccessRequestPromote promotes an access request to an access list.
func (c *Client) AccessRequestPromote(ctx context.Context, req *accesslistv1.AccessRequestPromoteRequest) (*accesslistv1.AccessRequestPromoteResponse, error) {
	resp, err := c.grpcClient.AccessRequestPromote(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// ListAccessListReviews will list access list reviews for a particular access list.
func (c *Client) ListAccessListReviews(ctx context.Context, accessList string, pageSize int, pageToken string) (reviews []*accesslist.Review, nextToken string, err error) {
	resp, err := c.grpcClient.ListAccessListReviews(ctx, &accesslistv1.ListAccessListReviewsRequest{
		AccessList: accessList,
		PageSize:   int32(pageSize),
		NextToken:  nextToken,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	reviews = make([]*accesslist.Review, len(resp.Reviews))
	for i, review := range resp.Reviews {
		var err error
		reviews[i], err = conv.FromReviewProto(review)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	return reviews, resp.GetNextToken(), nil
}

// ListAllAccessListReviews will list access list reviews for all access lists. Only to be used by the cache.
func (c *Client) ListAllAccessListReviews(ctx context.Context, pageSize int, pageToken string) (reviews []*accesslist.Review, nextToken string, err error) {
	resp, err := c.grpcClient.ListAllAccessListReviews(ctx, &accesslistv1.ListAllAccessListReviewsRequest{
		PageSize:  int32(pageSize),
		NextToken: nextToken,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	reviews = make([]*accesslist.Review, len(resp.Reviews))
	for i, review := range resp.Reviews {
		var err error
		reviews[i], err = conv.FromReviewProto(review)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	return reviews, resp.GetNextToken(), nil
}

// CreateAccessListReview will create a new review for an access list.
func (c *Client) CreateAccessListReview(ctx context.Context, review *accesslist.Review) (*accesslist.Review, time.Time, error) {
	resp, err := c.grpcClient.CreateAccessListReview(ctx, &accesslistv1.CreateAccessListReviewRequest{
		Review: conv.ToReviewProto(review),
	})
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}
	review.SetName(resp.ReviewName)
	return review, resp.NextAuditDate.AsTime(), nil
}

// DeleteAccessListReview will delete an access list review from the backend.
func (c *Client) DeleteAccessListReview(ctx context.Context, accessListName, reviewName string) error {
	_, err := c.grpcClient.DeleteAccessListReview(ctx, &accesslistv1.DeleteAccessListReviewRequest{
		AccessListName: accessListName,
		ReviewName:     reviewName,
	})
	return trace.Wrap(err)
}

// DeleteAllAccessListReviews will delete all access list reviews from all access lists.
func (c *Client) DeleteAllAccessListReviews(ctx context.Context) error {
	return trace.NotImplemented("DeleteAllAccessListReviews is not supported in the gRPC client")
}

// GetSuggestedAccessLists returns a list of access lists that are suggested for a given request.
func (c *Client) GetSuggestedAccessLists(ctx context.Context, accessRequestID string) ([]*accesslist.AccessList, error) {
	resp, err := c.grpcClient.GetSuggestedAccessLists(ctx, &accesslistv1.GetSuggestedAccessListsRequest{
		AccessRequestId: accessRequestID,
	})
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
