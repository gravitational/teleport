package provisioning

import (
	"context"
	"iter"

	"github.com/gravitational/trace"

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	usersv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/services"
)

// allProvisioningStates returns a sequence of (User, error) pairs that walks
// over the entire user list in the supplied UsersService. A non-nil error value
// indicates an error reading from the underlying Users data service , and no
// further Users will be yielded.
func allUsers(ctx context.Context, users UsersService) iter.Seq2[*types.UserV2, error] {
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

// allProvisioningStates returns a sequence of (PrincipalState, error) pairs that
// walks the entire list of principal states in the supplied provisioning data
// service. A non-nil error value indicates an error reading from the data
// service, and no further PrincipalStates will be yielded.
func allProvisioningStates(ctx context.Context, src services.DownstreamProvisioningStates, downstreamID services.DownstreamID) iter.Seq2[*provisioningv1.PrincipalState, error] {
	return clientutils.Resources(ctx, func(ctx context.Context, pageSize int, pageToken string) ([]*provisioningv1.PrincipalState, string, error) {
		return src.ListProvisioningStates2(ctx, downstreamID, pageSize, pageToken)
	})
}

// allAccessLists returns a sequence of (AccessList, error) pairs. A non-nil
// error value indicates an error reading from the access list service, and no
// further access lists wil be yielded.
func allAccessLists(ctx context.Context, acls AccessListsService) iter.Seq2[*accesslist.AccessList, error] {
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

// drainChannel returns a sequence that waits for a value on the supplied channel,
// and then pulls out as many items as it can until the read would block again,
// up to the supplied maximum number of items. The yielded values are a (T, bool)
// pair, with the boolean value indicating that the supplied T value is good (true),
// or if the drain was canceled and the T value is the type zero value.
//
// The drainChannel iterator will stop reading from the channel when:
//   - the supplied context is canceled
//   - a read from the channel (other than the initial read) would block
//   - the function has read `max` items out of the channel
//   - the callback function returns an error
func drainChannel[T any](ctx context.Context, ch <-chan T, max int) iter.Seq2[T, bool] {
	return func(yield func(T, bool) bool) {
		count := 0

		// Wait of the first item indefinitely (or at least as long as the supplied
		// context will let us)
		select {
		case <-ctx.Done():
			var t T
			yield(t, false)
			return

		case item, ok := <-ch:
			if !ok {
				var t T
				yield(t, false)
				return
			}
			if !yield(item, true) {
				return
			}

		}

		// Pull remaining items out of the channel until the specified maximum
		// is reached
		for count < max {
			select {
			case item, ok := <-ch:
				if !ok {
					var t T
					yield(t, false)
					return
				}
				if !yield(item, true) {
					return
				}
				count++

			case <-ctx.Done():
				var t T
				yield(t, false)
				return

			default:
				return
			}
		}
	}
}
