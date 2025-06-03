package resource

import (
	"context"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) getNamespace(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	return collections.NewNamespaceCollection([]types.Namespace{types.DefaultNamespace()}), nil
}
