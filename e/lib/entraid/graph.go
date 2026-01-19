package entraid

import (
	"context"

	"github.com/gravitational/teleport/lib/msgraph"
)

// GraphClient is an interface for interacting with the Microsoft Graph API.
type GraphClient interface {
	IterateUsers(ctx context.Context, f func(*msgraph.User) bool, opts ...msgraph.IterateOpt) error
	IterateGroups(ctx context.Context, f func(*msgraph.Group) bool, opts ...msgraph.IterateOpt) error
	IterateGroupMembers(ctx context.Context, groupID string, f func(msgraph.GroupMember) bool, opts ...msgraph.IterateOpt) error
	IterateGroupOwners(ctx context.Context, groupID string, f func(*msgraph.User) bool, opts ...msgraph.IterateOpt) error
	IterateApplications(ctx context.Context, f func(*msgraph.Application) bool, opts ...msgraph.IterateOpt) error
	GetApplication(ctx context.Context, appID string) (*msgraph.Application, error)
}
