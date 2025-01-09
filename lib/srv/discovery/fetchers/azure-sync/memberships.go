package azuresync

import (
	"context"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/msgraph"
)

const parallelism = 10

func expandMemberships(ctx context.Context, cli *msgraph.Client, principals []*accessgraphv1alpha.AzurePrincipal) ([]*accessgraphv1alpha.AzurePrincipal, error) { //nolint:unused // invoked in a dependent PR
	var eg errgroup.Group
	eg.SetLimit(parallelism)
	for _, principal := range principals {
		eg.Go(func() error {
			err := cli.IterateUserMembership(ctx, principal.Id, func(obj *msgraph.DirectoryObject) bool {
				principal.MemberOf = append(principal.MemberOf, *obj.ID)
				return true
			})
			if err != nil {
				return trace.Wrap(err)
			}
			return nil
		})
	}
	return principals, eg.Wait()
}
