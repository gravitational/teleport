package identitycenter

import (
	"context"

	"github.com/gravitational/teleport/api/client/proto"
	usersv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
)

// IntegrationsLister is an abstraction over a Integration (e.g. OIDC) data
// service, defining only the operations that the Identity Center integration
// needs.
type IntegrationsLister interface {
	GetIntegration(ctx context.Context, name string) (types.Integration, error)
}

// RolesService is an abstraction over a Role data service, defining only the
// operations that the Identity Center service needs.
type RolesService interface {
	// ListRoles returns a list of roles registered with the local
	ListRoles(context.Context, *proto.ListRolesRequest) (*proto.ListRolesResponse, error)
	// CreateRole creates a role, only if the role entry does not exist
	CreateRole(context.Context, types.Role) (types.Role, error)
	// GetRole returns role by name.
	GetRole(context.Context, string) (types.Role, error)
	// UpdateRole creates or updates a role and emits a related audit event.
	UpdateRole(context.Context, types.Role) (types.Role, error)
	// DeleteRole deletes a role by name
	DeleteRole(context.Context, string) error
}

// UsersLister defines an interface for iterating over the set of all Teleport
// users
type UsersLister interface {
	// ListUsers returns a list of users registered with the local
	// cluster auth server.
	ListUsers(ctx context.Context, req *usersv1.ListUsersRequest) (*usersv1.ListUsersResponse, error)
}

// UsersService is an abstraction over the user database used by the Identity Center
// service to manipulate user records
type UsersService interface {
	UsersLister
	// CreateUser creates a user, only if the user entry does not exist
	CreateUser(ctx context.Context, user types.User) (types.User, error)
	// GetUser returns a user by name.
	GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error)
	// UpdateUser updates the backend record to match the supplied struct,
	// returning the updated user.
	UpdateUser(ctx context.Context, user types.User) (types.User, error)
	// DeleteUser deletes the user with the given name
	DeleteUser(ctx context.Context, user string) error
}
