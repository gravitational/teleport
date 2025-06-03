package resource

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) getSemaphore(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	sems, err := client.GetSemaphores(ctx, types.SemaphoreFilter{
		SemaphoreKind: rc.ref.SubKind,
		SemaphoreName: rc.ref.Name,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewSemaphoreCollection(sems), nil
}
