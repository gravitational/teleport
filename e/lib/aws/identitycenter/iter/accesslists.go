package iter

import (
	"context"
	"iter"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/accesslist"
)

// AccessListLister is an interface for listing access lists
type AccessListLister interface {
	ListAccessLists(ctx context.Context, pageSize int, nextToken string) ([]*accesslist.AccessList, string, error)
}

// AllAccessLists returns a sequence of (AccessList, error) pairs. A non-nil
// error value indicates an error reading from the access list service, and no
// further access lists wil be yielded.
func AllAccessLists(ctx context.Context, acls AccessListLister) iter.Seq2[*accesslist.AccessList, error] {
	// TODO: find somewhere common for this to live. It seems generally useful.
	const (
		pageSize = 50
	)

	return func(yield func(*accesslist.AccessList, error) bool) {
		pageToken := ""
		for {
			acls, nextPage, err := acls.ListAccessLists(ctx, pageSize, pageToken)
			if err != nil {
				yield(nil, trace.Wrap(err, "listing access lists"))
				return
			}

			for _, acl := range acls {
				if !yield(acl, nil) {
					return
				}
			}

			if nextPage == "" {
				break
			}
			pageToken = nextPage
		}
	}
}
