package scim

import (
	"context"
	"crypto"

	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/services"
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

type IdentityService interface {
	GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error)
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
	ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error)

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

// CredentialsService is an abstraction over the cluster PluginsStaticCredentials
// database, defining only the operations used by the SCIM service
type CredentialsService interface {
	// GetPluginStaticCredentialsByLabels will get a list of plugin static credentials resource by matching labels.
	GetPluginStaticCredentialsByLabels(ctx context.Context, labels map[string]string) ([]types.PluginStaticCredentials, error)
}

// AccessListsService describes the subset of services.AccessLists required by
// the SCIM server
type AccessListsService interface {
	ListAccessLists(context.Context, int, string) ([]*accesslist.AccessList, string, error)

	// UpsertAccessList creates or updates an access list resource.
	UpsertAccessList(context.Context, *accesslist.AccessList) (*accesslist.AccessList, error)

	// GetAccessList fetches a specific access list by name
	GetAccessList(context.Context, string) (*accesslist.AccessList, error)

	// UpsertAccessListMember creates or updates an individual list AccessListMember
	// record
	UpsertAccessListMember(context.Context, *accesslist.AccessListMember) (*accesslist.AccessListMember, error)

	// UpsertAccessListWithMembers creates or updates an entire access list at
	// once
	UpsertAccessListWithMembers(context.Context, *accesslist.AccessList, []*accesslist.AccessListMember) (*accesslist.AccessList, []*accesslist.AccessListMember, error)

	// ListAccessListMembers pages over the known members of a given AccessList
	ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)

	// Delete deletes a specific AccessList membership
	DeleteAccessListMember(ctx context.Context, accessList string, memberName string) error

	// Delete deletes a specific AccessList. It is assumed that the delete will
	// cascades onto the AccessListMembers associated with the deleted list.
	DeleteAccessList(ctx context.Context, accessList string) error
}

// Static assertion that AccessListsService is a subset of services.AccessLists
// var _ AccessListsService = services.AccessLists(nil)

// RolesService describes the minimal set of operations that the SCIM service
// will perform on the cluster Role database. This is expected to be a
// subset of the Auth Service interface
type RolesService interface {
	// CreateRole creates a new role, failing if there is an existing Role of
	// the same name
	CreateRole(context.Context, types.Role) (types.Role, error)

	// DeleteRole deletes a given role
	DeleteRole(context.Context, string) error
}

// certAuthorityGetter is expected to be the Auth Server. Used by SCIM service to
// retrieve certificates for signing JWT tokens with [[jwtSigner]].
type certAuthorityGetter interface {
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)
}

// jwtSignerGetter is expected to be the KeyStore of the Auth Server. Used by the SCIM service to sign
// JWT tokens to authorize to third-party services like Okta. Certificates are provided with
// [[certAuthority]].
type jwtSignerGetter interface {
	GetJWTSigner(ctx context.Context, ca types.CertAuthority) (crypto.Signer, error)
}
