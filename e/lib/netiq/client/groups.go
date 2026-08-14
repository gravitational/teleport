package client

import (
	"context"

	"github.com/gravitational/trace"
)

// Group represents a group in the Identity Vault.
type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ListGroups returns a list of all groups in the Identity Vault.
func (c *Client) ListGroups(ctx context.Context) ([]Group, error) {
	const groupsBase = "rest/catalog/groups"

	groups, err := listResponse(ctx, c, groupsBase, func(r listGroupsResponse) ([]Group, error) {
		return r.Groups, nil
	})

	return groups, trace.Wrap(err)
}

type listGroupsResponse struct {
	Groups    []Group `json:"groups"`
	ArraySize int     `json:"arraySize"`
	NextIndex int     `json:"nextIndex"`
	Token     string  `json:"token"`
}

func (n listGroupsResponse) GetNextIndex() int {
	return n.NextIndex
}

// GroupMember represents a member of a group in the Identity Vault.
type GroupMember struct {
	// Name is the name of the member.
	Name string `json:"name"`
	// Dn is the distinguished name of the member.
	Dn string `json:"dn"`
	// IsGroupAssignment is a flag that determines whether the member is a group assignment.
	IsGroupAssignment bool `json:"isGroupAssignment"`
}

// ListGroupMembers returns a list of all members of the group with the given ID.
func (c *Client) ListGroupMembers(ctx context.Context, groupID string) ([]GroupMember, error) {
	b, err := newDNPayloadRequestBody(groupID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	const groupsMembersBase = "rest/access/groups/members"
	members, err := listResponse(
		ctx,
		c,
		groupsMembersBase,
		func(r listGroupsMembersResponse) ([]GroupMember, error) {
			return r.Recipients, nil
		},
		withQueryParams("q", "*"),
		withPostRequest(b),
	)

	return members, trace.Wrap(err)
}

type listGroupsMembersResponse struct {
	Recipients   []GroupMember `json:"recipients"`
	ArraySize    int           `json:"arraySize"`
	NextIndex    int           `json:"nextIndex"`
	CurrentIndex string        `json:"currentIndex"`
	EndIndex     string        `json:"endIndex"`
	Total        int           `json:"total"`
}

func (n listGroupsMembersResponse) GetNextIndex() int {
	return n.NextIndex
}
