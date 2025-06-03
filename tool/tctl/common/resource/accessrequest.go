package resource

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) getAccessRequest(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	resources, err := client.GetAccessRequests(ctx, types.AccessRequestFilter{ID: rc.ref.Name})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewAccessRequestCollection(resources), nil
}
