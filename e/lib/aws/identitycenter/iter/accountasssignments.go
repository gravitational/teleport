package iter

import (
	"context"
	"iter"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/pagination"
)

// AccountAssignmentLister is an abstraction over listing Account Assignments
type AccountAssignmentLister interface {
	// ListAccountAssignments lists all IdentityCenterAccountAssignment record
	// known to the service
	ListAccountAssignments(context.Context, int, *pagination.PageRequestToken) ([]services.IdentityCenterAccountAssignment, pagination.NextPageToken, error)
}

// AllAccountAssignments yields a sequence of (IdentityCenterAccountAssignment, error)
// pairs. If an error is encountered listing the account assignments then the
// sequence will yield a non-nil error value and the sequence will end
// immediately.
func AllAccountAssignments(ctx context.Context, svc AccountAssignmentLister) iter.Seq2[services.IdentityCenterAccountAssignment, error] {
	const pageSize = 20

	return func(yield func(services.IdentityCenterAccountAssignment, error) bool) {
		var pageToken pagination.PageRequestToken
		for {
			page, nextPage, err := svc.ListAccountAssignments(ctx, pageSize, &pageToken)
			if err != nil {
				yield(services.IdentityCenterAccountAssignment{}, trace.Wrap(err))
				return
			}

			for _, accountAssignment := range page {
				if !yield(accountAssignment, nil) {
					return
				}
			}

			if nextPage == pagination.EndOfList {
				break
			}
			pageToken.Update(nextPage)
		}
	}
}
