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
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

// AccessPoint is the Auth API used by the scim service.
type AccessPoint interface {
	Users
	Roles
	Identity
	Plugins
	Credentials
	JWTSigner
	types.Semaphores
	AccessLists
	services.AuthorityGetter
	Locks
}

// Config is the externally-supplied configuration data for the SCIM service.
type Config struct {
	// Authorizer is used to authorize requests to the SCIM service.
	Authorizer authz.Authorizer
	// Logger is the structured logger for the SCIM service.
	Logger *slog.Logger
	// AccessPoint groups service from auth.Server interface.
	// Any AccessPoint method may or may not be cached.
	AccessPoint
	// Backend groups service interfaces backed by auth.Server.Services
	// (direct storage backend reads). It is preferred to use AccessPoint if caching is not an issue.
	Backend Backend
	// Clock is a clock.
	Clock clockwork.Clock
	// HTTPClient is used to make HTTP requests.
	HTTPClient *http.Client
	// ClusterName is the name of the cluster this service is running in.
	ClusterName string
	// Modules defines build time constraints and licensed features.
	Modules modules.Modules
	// RateLimit overrides the default SCIM rate limits. Nil is
	// replaced by the standard defaults in [Config.CheckAndSetDefaults].
	// Intended for testing.
	RateLimit RateLimitConfig
}

func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.Authorizer == nil {
		return trace.BadParameter("missing authorizer")
	}
	if cfg.AccessPoint == nil {
		return trace.BadParameter("missing access point")
	}
	if cfg.Backend == nil {
		return trace.BadParameter("missing backend")
	}
	if cfg.Modules == nil {
		return trace.BadParameter("missing modules")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.HTTPClient == nil {
		httpClient, err := defaults.HTTPClient()
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.HTTPClient = httpClient
	}
	if cfg.ClusterName == "" {
		return trace.BadParameter("missing cluster name")
	}
	if cfg.RateLimit == (RateLimitConfig{}) {
		cfg.RateLimit = RateLimitConfig{
			Average:                 600,
			Burst:                   1200,
			PeriodSeconds:           60,
			MaxConcurrentOperations: 300,
		}
	}
	return nil
}

// Locks is an abstraction over the lock used by the SCIM service to
// manipulate Teleport user locks
type Locks interface {
	// GetLocks lists the locks that target a given set of resources.
	GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error)
	// UpsertLock creates or updates a given lock
	UpsertLock(ctx context.Context, lock types.Lock) error
	// DeleteLock deletes a given lock
	DeleteLock(ctx context.Context, name string) error
}

// Identity provides access to identity-related resources such as SAML
// connectors.
type Identity interface {
	GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error)
	GetClusterName(ctx context.Context) (types.ClusterName, error)
}

// Users is an abstraction over the user database used by the SCIM
// service to manipulate user records
type Users interface {
	// CreateUser creates a user, only if the user entry does not exist
	CreateUser(ctx context.Context, user types.User) (types.User, error)
	// GetUser returns a user by name.
	GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error)
	// UpdateUser updates the backend record to match the supplied struct,
	// returning the updated user.
	UpdateUser(ctx context.Context, user types.User) (types.User, error)
	// DeleteUser deletes the user with the given name
	DeleteUser(ctx context.Context, user string) error
	// ListUsers pages over the known users.
	ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error)
}

// Plugins is an abstraction over the plugins database used by the SCIM
// service to manipulate integration plugin records
type Plugins interface {
	GetPlugin(ctx context.Context, name string, withSecrets bool) (types.Plugin, error)
}

// Credentials is an abstraction over the cluster PluginsStaticCredentials
// database, defining only the operations used by the SCIM service
type Credentials interface {
	// GetPluginStaticCredentialsByLabels will get a list of plugin static credentials resource by matching labels.
	GetPluginStaticCredentialsByLabels(ctx context.Context, labels map[string]string) ([]types.PluginStaticCredentials, error)
}

// AccessLists describes the subset of services.AccessLists required by
// the SCIM server.
type AccessLists interface {
	// ListAccessLists pages over the known access lists.
	ListAccessLists(context.Context, int, string) ([]*accesslist.AccessList, string, error)
	// GetAccessList fetches a specific access list by name.
	GetAccessList(context.Context, string) (*accesslist.AccessList, error)
	// GetAccessListMember fetches a specific access list member by name.
	GetAccessListMember(ctx context.Context, accessListName string, memberName string) (*accesslist.AccessListMember, error)
	// ListAccessListMembers pages over the known members of a given AccessList.
	ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	// UpsertAccessList creates or updates an access list resource.
	UpsertAccessList(context.Context, *accesslist.AccessList) (*accesslist.AccessList, error)
	// UpsertAccessListMember creates or updates an individual list AccessListMember
	// record
	UpsertAccessListMember(context.Context, *accesslist.AccessListMember) (*accesslist.AccessListMember, error)
	// UpsertAccessListWithMembers creates or updates an entire access list at
	// once
	UpsertAccessListWithMembers(context.Context, *accesslist.AccessList, []*accesslist.AccessListMember) (*accesslist.AccessList, []*accesslist.AccessListMember, error)
	// UpdateAccessListAndOverwriteMembers conditionally updates the access list,
	// overwriting the list's members if successful.
	UpdateAccessListAndOverwriteMembers(context.Context, *accesslist.AccessList, []*accesslist.AccessListMember) (*accesslist.AccessList, []*accesslist.AccessListMember, error)
	// DeleteAccessListMember deletes a specific AccessList membership
	DeleteAccessListMember(ctx context.Context, accessList string, memberName string) error
	// DeleteAccessList deletes a specific AccessList. It is assumed that the delete will
	// cascade onto the AccessListMembers associated with the deleted list.
	DeleteAccessList(ctx context.Context, accessList string) error
}

// Backend groups the service interfaces backed by auth.Server.Services
// direct backend reads
type Backend interface {
	AccessLists
	Assignments
	Locks
}

// Roles describes the minimal set of operations that the SCIM service
// will perform on the cluster Role database. This is expected to be a
// subset of the Auth Service interface
type Roles interface {
	// CreateRole creates a new role, failing if there is an existing Role of
	// the same name
	CreateRole(context.Context, types.Role) (types.Role, error)
	// DeleteRole deletes a given role
	DeleteRole(context.Context, string) error
}

// JWTSigner is expected to be the KeyStore of the Auth Server. Used by the
// SCIM service to sign JWT tokens to authorize to third-party services like
// Okta.
type JWTSigner interface {
	GetJWTSigner(ctx context.Context, ca types.CertAuthority) (crypto.Signer, error)
}

// Assignments is used by the SCIM service to fetch Okta assignments.
type Assignments interface {
	// ListOktaAssignments lists Okta assignments.
	ListOktaAssignments(context.Context, int, string) ([]types.OktaAssignment, string, error)
}
