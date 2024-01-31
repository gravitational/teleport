package scim

import (
	"context"

	"github.com/gravitational/teleport/api/types"
)

// UsersService is an abstraction over the lock used by the SCIM service to
// manipulate Teleport user locks
type LocksService interface {
	// GetLocks lists the locks that target a given set of resources.
	GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error)

	// UpsertLock creates or updates a given lock
	UpsertLock(ctx context.Context, lock types.Lock) error

	// DeleteLock deletes a given lock
	DeleteLock(ctx context.Context, name string) error
}

// UsersService is an abstraction over the user database used by the SCIM
// service to manipulate user records
type UsersService interface {
	// CreateUserWithContext creates a user, only if the user entry does not exist
	CreateUser(ctx context.Context, user types.User) (types.User, error)

	// GetUserWithContext returns a user by name.
	GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error)

	// ListUsers returns a list of users registered with the local
	// cluster auth server.
	ListUsers(ctx context.Context, pageSize int, nextToken string, withSecrets bool) ([]types.User, string, error)

	// UpdateUser updates the backend record to match the supplied struct,
	// returning the updated user.
	UpdateUser(ctx context.Context, user types.User) (types.User, error)

	// DeleteUser deletes the user with the given name
	DeleteUser(ctx context.Context, user string) error
}

// UsersService is an abstraction over the plugins database used by the SCIM
// service to manipulate integration plugin records
type PluginsService interface {
	GetPlugin(ctx context.Context, name string, withSecrets bool) (types.Plugin, error)
}

type CredentialsService interface {
	// GetPluginStaticCredentialsByLabels will get a list of plugin static credentials resource by matching labels.
	GetPluginStaticCredentialsByLabels(ctx context.Context, labels map[string]string) ([]types.PluginStaticCredentials, error)
}
