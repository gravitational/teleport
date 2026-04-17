package entraid

import (
	"context"

	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/teleport/lib/msgraph/models"
)

// GraphClient is an interface for interacting with the Microsoft Graph API.
type GraphClient interface {
	IterateUsers(ctx context.Context, f func(*models.User) bool, opts ...msgraph.IterateOpt) error
	IterateGroups(ctx context.Context, f func(*models.Group) bool, opts ...msgraph.IterateOpt) error
	IterateGroupMembers(ctx context.Context, groupID string, f func(models.GroupMember) bool, opts ...msgraph.IterateOpt) error
	IterateGroupOwners(ctx context.Context, groupID string, f func(*models.User) bool, opts ...msgraph.IterateOpt) error
	IterateApplications(ctx context.Context, f func(*models.Application) bool, opts ...msgraph.IterateOpt) error
	GetApplication(ctx context.Context, appID string) (*models.Application, error)
}
