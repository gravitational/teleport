package resource

import (
	"context"
	"fmt"

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

func (rc *ResourceCommand) deleteSemaphore(ctx context.Context, client *authclient.Client) error {
	if rc.ref.SubKind == "" || rc.ref.Name == "" {
		return trace.BadParameter(
			"full semaphore path must be specified (e.g. '%s/%s/alice@example.com')",
			types.KindSemaphore, types.SemaphoreKindConnection,
		)
	}
	err := client.DeleteSemaphore(ctx, types.SemaphoreFilter{
		SemaphoreKind: rc.ref.SubKind,
		SemaphoreName: rc.ref.Name,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("semaphore '%s/%s' has been deleted\n", rc.ref.SubKind, rc.ref.Name)
	return nil
}
