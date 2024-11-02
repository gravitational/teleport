package entraid

import (
	"context"

	"github.com/gravitational/trace"

	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/msgraph"
)

type userAccessPoint interface {
	CreateUser(ctx context.Context, user types.User) (types.User, error)
	ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error)
	DeleteUser(ctx context.Context, user string) error
	UpdateUser(ctx context.Context, user types.User) (types.User, error)
}

type accessListAccessPoint interface {
	ListAccessLists(context.Context, int, string) ([]*accesslist.AccessList, string, error)
	UpsertAccessList(context.Context, *accesslist.AccessList) (*accesslist.AccessList, error)
	DeleteAccessList(context.Context, string) error

	ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	UpsertAccessListMember(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error)
	DeleteAccessListMember(ctx context.Context, accessList string, memberName string) error
}

type samlService interface {
	GetSAMLConnector(ctx context.Context, name string, withSecrets bool) (types.SAMLConnector, error)
}

// DirectoryReconciler uses the Microsoft Graph API
// to synchronize Entra ID users and groups into the Teleport cluster
// as users and access lists.
type DirectoryReconciler struct {
	graphClient   graphClient
	userSvc       userAccessPoint
	samlService   samlService
	accessListSvc accessListAccessPoint

	// defaultOwners specifies the default owners for access lists synchronized from Entra ID.
	defaultOwners []accesslist.Owner
	// ssoConnectorID specifies the ID (name) of the Auth connector that imported users are associated with
	ssoConnectorID string
	// tenantID specifies the Entra Tenant ID
	tenantID string

	// importedUsers is the number of users imported as of the most recent reconciliation.
	// If reconciling users fails, this number is not updated.
	importedUsers int

	// importedGroups is the number of groups imported as of the most recent reconciliation.
	// If reconciling groups fails, this number is not updated.
	importedGroups int
}

// DirectoryReconcilerConfig specifies dependencies and parameters for instantiating DirectoryReconciler.
type DirectoryReconcilerConfig struct {
	// GraphClient is the instantiated Microsoft Graph API client.
	GraphClient *msgraph.Client
	// UserSvc is the service used to read and modify Teleport users.
	UserSvc userAccessPoint
	// AccessListSvc is the service used to read and modify Teleport access lists.
	AccessListSvc accessListAccessPoint
	// SAMLSvc is the service used to read SAML connectors.
	SAMLSvc samlService

	// DefaultOwners specifies the default owners for access lists synchronized from Entra ID.
	DefaultOwners []accesslist.Owner
	// SSOConnectorID specifies the ID (name) of the Auth connector that imported users are associated with
	SSOConnectorID string
	// TenantID specifies the Entra Tenant ID
	TenantID string
}

// Validate ensures that required values are set.
func (cfg *DirectoryReconcilerConfig) Validate() error {
	if cfg.GraphClient == nil {
		return trace.BadParameter("GraphClient is required")
	}
	if cfg.UserSvc == nil {
		return trace.BadParameter("UserSvc is required")
	}
	if cfg.AccessListSvc == nil {
		return trace.BadParameter("AccessListSvc is required")
	}
	if len(cfg.DefaultOwners) == 0 {
		return trace.BadParameter("DefaultOwners is required")
	}
	if cfg.SSOConnectorID == "" {
		return trace.BadParameter("SSOConnectorID is required")
	}
	if cfg.TenantID == "" {
		return trace.BadParameter("TenantID is required")
	}

	if cfg.SAMLSvc == nil {
		return trace.BadParameter("SAMLSvc is required")
	}

	return nil
}

// NewDirectoryReconciler creates a new DirectoryReconciler using the given config.
func NewDirectoryReconciler(cfg DirectoryReconcilerConfig) (*DirectoryReconciler, error) {
	if err := cfg.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &DirectoryReconciler{
		graphClient:   cfg.GraphClient,
		userSvc:       cfg.UserSvc,
		accessListSvc: cfg.AccessListSvc,
		defaultOwners: cfg.DefaultOwners,
		tenantID:      cfg.TenantID,
		samlService:   cfg.SAMLSvc,
	}, nil
}

// Reconcile does a one-time reconciliation of users and access lists
// from Entra ID to Teleport.
func (r *DirectoryReconciler) Reconcile(ctx context.Context) error {
	groupsMap, err := listEntraGroups(ctx, r.graphClient)
	if err != nil {
		return trace.Wrap(err, "failed to list Entra ID groups")
	}

	groupMembersMap, err := listEntraGroupsMembers(ctx, r.graphClient, groupsMap)
	if err != nil {
		return trace.Wrap(err, "failed to list Entra ID group members")
	}
	usersByEntraID, err := r.reconcileUsers(ctx, groupsMap, groupMembersMap)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := r.reconcileAccessLists(ctx, usersByEntraID, groupsMap, groupMembersMap); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ImportedUsers returns the total number of users imported as of the most recent reconciliation.
func (r *DirectoryReconciler) ImportedUsers() int {
	return r.importedUsers
}

// ImportedUsers returns the total number of groups imported as of the most recent reconciliation.
func (r *DirectoryReconciler) ImportedGroups() int {
	return r.importedGroups
}

func matchByLabel[T types.Resource](resource T) bool {
	origin, ok := resource.GetMetadata().Labels[types.OriginLabel]
	return ok && origin == types.OriginEntraID
}
