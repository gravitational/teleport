package entraid

import (
	"context"

	"github.com/gravitational/teleport/lib/msgraph"
)

type graphClient interface {
	IterateUsers(ctx context.Context, f func(*msgraph.User) bool) error
	IterateGroups(ctx context.Context, f func(*msgraph.Group) bool) error
	IterateGroupMembers(ctx context.Context, groupID string, f func(msgraph.GroupMember) bool) error
	IterateApplications(ctx context.Context, f func(*msgraph.Application) bool) error
}
