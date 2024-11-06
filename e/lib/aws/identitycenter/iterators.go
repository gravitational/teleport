package identitycenter

import (
	"context"
	"iter"

	"github.com/gravitational/trace"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/pagination"
)

type permissionSetLister interface {
	ListPermissionSets(context.Context, int, *pagination.PageRequestToken) ([]*identitycenterv1.PermissionSet, pagination.NextPageToken, error)
}

func allPermissionSets(ctx context.Context, src permissionSetLister) iter.Seq2[*identitycenterv1.PermissionSet, error] {
	const pageSize = 50

	return func(yield func(*identitycenterv1.PermissionSet, error) bool) {
		var pageToken pagination.PageRequestToken
		for {
			pss, nextPage, err := src.ListPermissionSets(ctx, pageSize, &pageToken)
			if err != nil {
				yield(nil, trace.Wrap(err, "iterating over all permission sets"))
				return
			}

			for _, ps := range pss {
				if !yield(ps, nil) {
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

func allAccountAssignments(ctx context.Context, src services.IdentityCenterAccountAssignments) iter.Seq2[services.IdentityCenterAccountAssignment, error] {
	const pageSize = 100

	return func(yield func(services.IdentityCenterAccountAssignment, error) bool) {
		var pageToken pagination.PageRequestToken
		for {
			pageItems, nextPage, err := src.ListAccountAssignments(ctx, pageSize, &pageToken)
			if err != nil {
				yield(services.IdentityCenterAccountAssignment{},
					trace.Wrap(err, "enumerating identity center account assignment resources"))
				return
			}

			for _, asmt := range pageItems {
				if !yield(asmt, nil) {
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

func allAccounts(ctx context.Context, src services.IdentityCenterAccounts) iter.Seq2[services.IdentityCenterAccount, error] {
	const pageSize = 100

	return func(yield func(services.IdentityCenterAccount, error) bool) {
		var pageToken pagination.PageRequestToken
		for {
			pageItems, nextPage, err := src.ListIdentityCenterAccounts(ctx, pageSize, &pageToken)
			if err != nil {
				yield(services.IdentityCenterAccount{},
					trace.Wrap(err, "enumerating identity center account resources"))
				return
			}

			for _, asmt := range pageItems {
				if !yield(asmt, nil) {
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
