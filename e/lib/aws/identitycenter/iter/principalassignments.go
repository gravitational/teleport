package iter

import (
	"context"
	"iter"

	"github.com/gravitational/trace"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/lib/utils/pagination"
)

type PrincipalAssignmentLister interface {
	ListPrincipalAssignments(context.Context, int, *pagination.PageRequestToken) ([]*identitycenterv1.PrincipalAssignment, pagination.NextPageToken, error)
}

func AllPrincipalAssignments(ctx context.Context, src PrincipalAssignmentLister) iter.Seq2[*identitycenterv1.PrincipalAssignment, error] {
	return func(yield func(*identitycenterv1.PrincipalAssignment, error) bool) {
		const pageSize = 25

		var pageToken pagination.PageRequestToken
		for {
			page, nextPage, err := src.ListPrincipalAssignments(ctx, pageSize, &pageToken)
			if err != nil {
				yield(nil, trace.Wrap(err))
				return
			}
			for _, pa := range page {
				if !yield(pa, nil) {
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
