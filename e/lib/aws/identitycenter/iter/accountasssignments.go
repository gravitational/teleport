package iter

import (
	"context"
	"iter"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/utils/clientutils"
)

// AccountAssignmentLister is an abstraction over listing Account Assignments
type AccountAssignmentLister interface {
	// ListIdentityCenterAccountAssignments lists all IdentityCenterAccountAssignment record
	// known to the service
	ListIdentityCenterAccountAssignments(context.Context, int, string) ([]*identitycenterv1.AccountAssignment, string, error)
}

// AllAccountAssignments yields a sequence of (IdentityCenterAccountAssignment, error)
// pairs. If an error is encountered listing the account assignments then the
// sequence will yield a non-nil error value and the sequence will end
// immediately.
func AllAccountAssignments(ctx context.Context, svc AccountAssignmentLister) iter.Seq2[*identitycenterv1.AccountAssignment, error] {
	return clientutils.Resources(ctx, svc.ListIdentityCenterAccountAssignments)
}
