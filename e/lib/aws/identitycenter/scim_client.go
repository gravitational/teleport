package identitycenter

import (
	"context"

	"github.com/gravitational/trace"

	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
)

// icSCIMClient wraps a SCIM client, overriding ListGroupMembers to use the AWS
// Identity Center API. This is necessary because the AWS IC SCIM Groups endpoint
// does not list members when queried, so we have to find the member list another
// way.
type icSCIMClient struct {
	scimsdk.Client
	icClient icsdk.Client
}

// newICSCIMClient creates a SCIM client wrapper that uses the supplied IC API
// client to list group members instead of SCIM.
func newICSCIMClient(scimClient scimsdk.Client, icClient icsdk.Client) scimsdk.Client {
	return &icSCIMClient{Client: scimClient, icClient: icClient}
}

// ListGroupMembers uses the AWS Identity Center API to list the current members
// of a group. This provides accurate membership data that is not available via
// the SCIM API.
func (c *icSCIMClient) ListGroupMembers(ctx context.Context, id string) ([]*scimsdk.GroupMember, error) {
	icMembers, err := c.icClient.ListGroupMemberships(ctx, id)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	members := make([]*scimsdk.GroupMember, len(icMembers))
	for i, m := range icMembers {
		members[i] = &scimsdk.GroupMember{
			ExternalID: m.MemberID,
			Type:       scimsdk.ResourceTypeUser,
		}
	}
	return members, nil
}
