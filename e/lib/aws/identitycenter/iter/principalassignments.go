package iter

import (
	"context"
	"iter"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/utils/clientutils"
)

type PrincipalAssignmentLister interface {
	ListPrincipalAssignments(context.Context, int, string) ([]*identitycenterv1.PrincipalAssignment, string, error)
}

func AllPrincipalAssignments(ctx context.Context, src PrincipalAssignmentLister) iter.Seq2[*identitycenterv1.PrincipalAssignment, error] {
	return clientutils.Resources(ctx, src.ListPrincipalAssignments)
}
