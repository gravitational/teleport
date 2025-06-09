package resource

import (
	"context"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var namespace = resource{
	getHandler: getNamespace,
}

func getNamespace(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	return collections.NewNamespaceCollection([]types.Namespace{types.DefaultNamespace()}), nil
}
