package common

import (
	"context"
	"crypto"
	"log/slog"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/e/lib/okta/common"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

// Config is the externally-supplied configuration data for the SCIM service.
type Config struct {
	// Authorizer is used to authorize requests to the SCIM service.
	Authorizer authz.Authorizer
	// FilterParser is used to parse SCIM filters.
	Logger *slog.Logger
	// UsersService is used to manage users.
	UsersService UsersService
	// RolesService is used to manage roles.
	RolesService RolesService
	// PluginsService is used to manage plugins.
	PluginsService PluginsService
	// CredentialsService is used to manage plugin credentials.
	CredentialsService CredentialsService
	// AccessListsService is used to manage access lists.
	AccessListsService AccessListsService
	// CertAuthorityGetter is used to retrieve certificates for signing JWT tokens.
	CertAuthorityGetter CertAuthorityGetter
	// JWTSignerGetter is used to sign JWT tokens.
	JWTSignerGetter JWTSignerGetter
	// LocksService is used to manage locks.
	LocksService LocksService
	// HTTPClient is used to make HTTP requests.
	AuthorityGetter services.AuthorityGetter
	// IdentityService is used to manage identity providers.
	Clock clockwork.Clock
	// IdentityService is used to manage identity providers.
	IdentityService IdentityService
	// UserGetter is used to get user information.
	HTTPClient *http.Client
	// UserGetter is used to get user information.
	UserGetter services.UserGetter
	// AccessListGetter is used to get access list information.
	AccessListGetter accessListGetter
	// AssignmentService is used to fetch Okta assignments.
	AssignmentService common.OktaAssignmentService
	// ClusterName is the name of the cluster this service is running in.
	ClusterName string
}
type accessListGetter interface {
	GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error)
	ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
}

func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.Authorizer == nil {
		return trace.BadParameter("missing authorizer")
	}
	if cfg.UsersService == nil {
		return trace.BadParameter("missing user service")
	}
	if cfg.RolesService == nil {
		return trace.BadParameter("missing roles service")
	}
	if cfg.PluginsService == nil {
		return trace.BadParameter("missing plugin service")
	}
	if cfg.LocksService == nil {
		return trace.BadParameter("missing locks service")
	}
	if cfg.AccessListsService == nil {
		return trace.BadParameter("missing access lists service")
	}
	if cfg.CertAuthorityGetter == nil {
		return trace.BadParameter("missing cert authority")
	}
	if cfg.JWTSignerGetter == nil {
		return trace.BadParameter("missing JWT signer")
	}
	if cfg.IdentityService == nil {
		return trace.BadParameter("missing identity service")
	}
	if cfg.CredentialsService == nil {
		return trace.BadParameter("missing plugin credentials service")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.UserGetter == nil {
		cfg.UserGetter = cfg.UsersService
	}
	if cfg.AccessListGetter == nil {
		cfg.AccessListGetter = cfg.AccessListsService
	}
	if cfg.HTTPClient == nil {
		httpClient, err := defaults.HTTPClient()
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.HTTPClient = httpClient
	}
	if cfg.AssignmentService == nil {
		return trace.BadParameter("missing assignment service")
	}

	if cfg.ClusterName == "" {
		return trace.BadParameter("missing cluster name")
	}
	return nil
}

// LocksService is an abstraction over the lock used by the SCIM service to
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
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadSigningKeys bool) (types.CertAuthority, error)
}

// UsersService is an abstraction over the user database used by the SCIM
// service to manipulate user records
type UsersService interface {
	// CreateUser creates a user, only if the user entry does not exist
	CreateUser(ctx context.Context, user types.User) (types.User, error)
	// GetUser returns a user by name.
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

// PluginsService is an abstraction over the plugins database used by the SCIM
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
	// UpdateAccessListAndOverwriteMembers conditionally updates the access list,
	// overwriting the list's members if successful.
	UpdateAccessListAndOverwriteMembers(context.Context, *accesslist.AccessList, []*accesslist.AccessListMember) (*accesslist.AccessList, []*accesslist.AccessListMember, error)

	// ListAccessListMembers pages over the known members of a given AccessList
	ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	// DeleteAccessListMember deletes a specific AccessList membership
	DeleteAccessListMember(ctx context.Context, accessList string, memberName string) error
	// DeleteAccessList deletes a specific AccessList. It is assumed that the delete will
	// cascade onto the AccessListMembers associated with the deleted list.
	DeleteAccessList(ctx context.Context, accessList string) error
	// GetAccessListMember fetches a specific access list member by name
	GetAccessListMember(ctx context.Context, accessListName string, memberName string) (*accesslist.AccessListMember, error)
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

// CertAuthorityGetter is expected to be the Auth Server. Used by SCIM service to
// retrieve certificates for signing JWT tokens with [[jwtSigner]].
type CertAuthorityGetter interface {
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)
	GetClusterName(ctx context.Context) (types.ClusterName, error)
}

// JWTSignerGetter is expected to be the KeyStore of the Auth Server. Used by the SCIM service to sign
// JWT tokens to authorize to third-party services like Okta. Certificates are provided with
// [[certAuthority]].
type JWTSignerGetter interface {
	GetJWTSigner(ctx context.Context, ca types.CertAuthority) (crypto.Signer, error)
}
