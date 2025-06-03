package resource

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) getReverseTunnel(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name != "" {
		return nil, trace.BadParameter("reverse tunnel cannot be searched by name")
	}
	var tunnels []types.ReverseTunnel
	var nextToken string
	for {
		var page []types.ReverseTunnel
		var err error

		const defaultPageSize = 0
		page, nextToken, err = client.ListReverseTunnels(ctx, defaultPageSize, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tunnels = append(tunnels, page...)
		if nextToken == "" {
			break
		}
	}
	return collections.NewReverseTunnelCollection(tunnels), nil
}
