package iter

import (
	"context"
	"iter"

	"github.com/gravitational/trace"

	usersv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
)

// UserLister defines an itnerface for listing Users.
type UserLister interface {
	ListUsers(ctx context.Context, req *usersv1.ListUsersRequest) (*usersv1.ListUsersResponse, error)
}

// allUsers returns a sequence of (User, error) pairs that walks over the entire
// user list in the supplied UsersService. A non-nil error value indicates an
// error reading from the underlying Users data service, and no further Users
// will be yielded.
func AllUsers(ctx context.Context, users UserLister) iter.Seq2[*types.UserV2, error] {
	// TODO: find somewhere common for this to live. It seems generally useful.
	const (
		pageSize = 50
	)

	return func(yield func(*types.UserV2, error) bool) {
		pageToken := ""
		for {
			response, err := users.ListUsers(ctx, usersv1.ListUsersRequest_builder{
				PageSize:  pageSize,
				PageToken: pageToken,
			}.Build())
			if err != nil {
				yield(nil, trace.Wrap(err, "listing users"))
				return
			}

			for _, u := range response.GetUsers() {
				if !yield(u, nil) {
					return
				}
			}

			if response.GetNextPageToken() == "" {
				break
			}
			pageToken = response.GetNextPageToken()
		}
	}
}
