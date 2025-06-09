package resource

import (
	"context"
	"fmt"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var semaphore = resource{
	getHandler:    getSemaphore,
	deleteHandler: deleteSemaphore,
}

func getSemaphore(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	sems, err := client.GetSemaphores(ctx, types.SemaphoreFilter{
		SemaphoreKind: ref.SubKind,
		SemaphoreName: ref.Name,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewSemaphoreCollection(sems), nil
}

func deleteSemaphore(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if ref.SubKind == "" || ref.Name == "" {
		return trace.BadParameter(
			"full semaphore path must be specified (e.g. '%s/%s/alice@example.com')",
			types.KindSemaphore, types.SemaphoreKindConnection,
		)
	}
	err := client.DeleteSemaphore(ctx, types.SemaphoreFilter{
		SemaphoreKind: ref.SubKind,
		SemaphoreName: ref.Name,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("semaphore '%s/%s' has been deleted\n", ref.SubKind, ref.Name)
	return nil
}
