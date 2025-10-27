package entraid

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"

	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/teleport/lib/plugins/filter"
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

	UpsertAccessListWithMembers(ctx context.Context, accessList *accesslist.AccessList, membersIn []*accesslist.AccessListMember) (*accesslist.AccessList, []*accesslist.AccessListMember, error)

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
	clock           clockwork.Clock
	logger          *slog.Logger
	metricsRegistry prometheus.Registerer
	metrics         *directoryMetrics
	graphClient     GraphClient
	userSvc         userAccessPoint
	samlService     samlService
	accessListSvc   accessListAccessPoint

	// defaultOwners specifies the default owners for access lists synchronized from Entra ID.
	defaultOwners []accesslist.Owner
	// ssoConnectorID specifies the ID (name) of the Auth connector that imported users are associated with
	ssoConnectorID string
	// entraAppID specifies the Entra Application ID for the SAML connector
	entraAppID string

	// tenantID specifies the Entra Tenant ID
	tenantID string

	// importedUsers is the number of users imported as of the most recent reconciliation.
	// If reconciling users fails, this number is not updated.
	importedUsers int

	// importedGroups is the number of groups imported as of the most recent reconciliation.
	// If reconciling groups fails, this number is not updated.
	importedGroups int
	// groupsFilter specifies Entra ID group filters.
	groupsFilter filter.Filters
}

// DirectoryReconcilerConfig specifies dependencies and parameters for instantiating DirectoryReconciler.
type DirectoryReconcilerConfig struct {
	Clock  clockwork.Clock
	Logger *slog.Logger
	// MetricsRegistry is used to register metrics. When nil, metrics are not registered.
	MetricsRegistry prometheus.Registerer
	// GraphClient is the instantiated Microsoft Graph API client.
	GraphClient GraphClient
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
	// EntraAppID specifies the Entra Application ID for the SAML connector
	EntraAppID string
	// TenantID specifies the Entra Tenant ID
	TenantID string
	// GroupsFilter specifies Entra ID group filters.
	GroupsFilter filter.Filters
}

// Validate ensures that required values are set.
func (cfg *DirectoryReconcilerConfig) Validate() error {
	if cfg.Clock == nil {
		return trace.BadParameter("Clock is required")
	}
	if cfg.Logger == nil {
		return trace.BadParameter("Logger is required")
	}
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

	if cfg.EntraAppID == "" {
		return trace.BadParameter("EntraAppID is required")
	}

	return nil
}

// NewDirectoryReconciler creates a new DirectoryReconciler using the given config.
func NewDirectoryReconciler(cfg DirectoryReconcilerConfig) (*DirectoryReconciler, error) {
	if err := cfg.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}

	metrics := newMetrics()
	// gracefully handle not being given a metric registry
	if cfg.MetricsRegistry != nil {
		if err := metrics.register(cfg.MetricsRegistry); err != nil {
			return nil, trace.Wrap(err, "registering metrics")
		}
	}

	return &DirectoryReconciler{
		clock:           cfg.Clock,
		logger:          cfg.Logger,
		metricsRegistry: cfg.MetricsRegistry,
		graphClient:     cfg.GraphClient,
		userSvc:         cfg.UserSvc,
		accessListSvc:   cfg.AccessListSvc,
		defaultOwners:   cfg.DefaultOwners,
		tenantID:        cfg.TenantID,
		samlService:     cfg.SAMLSvc,
		ssoConnectorID:  cfg.SSOConnectorID,
		entraAppID:      cfg.EntraAppID,
		groupsFilter:    cfg.GroupsFilter,
		metrics:         metrics,
	}, nil
}

// Reconcile does a one-time reconciliation of users and access lists
// from Entra ID to Teleport.
func (r *DirectoryReconciler) Reconcile(ctx context.Context) (err error) {
	defer func() {
		r.metrics.reconciliationCount.With(prometheus.Labels{
			metricLabelResult: metricLabelResultFromError(err),
		}).Inc()
	}()

	reconciliationStart := r.clock.Now()
	useLocalGroupMatcher := false
	entraGroupMatcher := groupFilterMatcher(r.groupsFilter)
	if _, err := filter.New(r.groupsFilter); err != nil {
		switch {
		case errors.Is(err, filter.ErrUnknownFilter):
			// If the configured filter has an unknown filter type,
			// which may happen during cluster downgrade where an older cluster
			// may not understand the newer filter type, the reconciliation
			// should only apply to the already-synced groups.
			useLocalGroupMatcher = true
			r.logger.ErrorContext(ctx, "Unknown group filter encountered, filters will be skipped and reconciliation will be limited to the existing Entra ID groups", "error", err.Error())
		default:
			return trace.Wrap(err)
		}
	}

	start := r.clock.Now()
	teleportAccessListsWithMembersMap, err := listTeleportAccessListsWithMembers(ctx, r.accessListSvc)
	if err != nil {
		return trace.Wrap(err)
	}
	took := r.clock.Since(start)
	r.metrics.reconciliationDuration.With(prometheus.Labels{
		metricLabelSection: "read_list_backend",
	}).Observe(took.Seconds())

	if useLocalGroupMatcher {
		entraGroupMatcher = groupLocalMatcher(teleportAccessListsWithMembersMap)
	}

	start = r.clock.Now()
	groupsMap, err := listEntraGroups(ctx, r.graphClient, entraGroupMatcher)
	if err != nil {
		return trace.Wrap(err)
	}
	took = r.clock.Since(start)
	r.logger.DebugContext(ctx, "Finished listing Entra ID groups", "took", took.String())
	r.metrics.reconciliationDuration.With(prometheus.Labels{
		metricLabelSection: "read_entra_groups",
	}).Observe(took.Seconds())
	r.metrics.discoveredEntraGroups.Set(float64(len(groupsMap)))

	start = r.clock.Now()
	groupMembersMap, err := listEntraGroupsMembers(ctx, r.graphClient, groupsMap)
	if err != nil {
		return trace.Wrap(err)
	}
	took = r.clock.Since(start)
	r.logger.DebugContext(ctx, "Finished listing Entra ID group members", "took", took.String())
	r.metrics.reconciliationDuration.With(prometheus.Labels{
		metricLabelSection: "read_entra_members",
	}).Observe(took.Seconds())
	r.metrics.discoveredEntraMemberships.Set(float64(len(groupMembersMap)))

	start = r.clock.Now()
	usersByEntraID, err := r.reconcileUsers(ctx, groupsMap, groupMembersMap)
	if err != nil {
		return trace.Wrap(err)
	}
	took = r.clock.Since(start)
	r.logger.DebugContext(ctx, "Finished reconciling Entra ID and Teleport users", "took", took.String())
	r.metrics.reconciliationDuration.With(prometheus.Labels{
		metricLabelSection: "reconcile_users",
	}).Observe(took.Seconds())

	start = r.clock.Now()
	if err := r.reconcileAccessLists(ctx,
		usersByEntraID,
		groupsMap, groupMembersMap,
		teleportAccessListsWithMembersMap,
	); err != nil {
		return trace.Wrap(err)
	}
	took = r.clock.Since(start)
	r.logger.DebugContext(ctx, "Finished reconciling Entra ID groups and Teleport access lists", "took", took.String())
	r.metrics.reconciliationDuration.With(prometheus.Labels{
		metricLabelSection: "reconcile_access_lists",
	}).Observe(took.Seconds())
	r.metrics.reconciliationDuration.With(prometheus.Labels{
		metricLabelSection: "total",
	}).Observe(r.clock.Since(reconciliationStart).Seconds())

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

func (r *DirectoryReconciler) getApplication(ctx context.Context, appID string) (*msgraph.Application, error) {
	app, err := r.graphClient.GetApplication(ctx, appID)
	return app, trace.Wrap(err, "failed to get application")
}

func getGroupNameBuilderFunc(app *msgraph.Application) (bool, func(*msgraph.Group) string) {
	getGroupID := func(group *msgraph.Group) string {
		if group.ID == nil {
			return ""
		}
		return *group.ID
	}
	if app.OptionalClaims == nil || len(app.OptionalClaims.SAML2Token) == 0 {
		return false, getGroupID
	}

	var groupAditionalProperties []string
	for _, claim := range app.OptionalClaims.SAML2Token {
		if claim.Name == nil || *claim.Name != msgraph.OPTIONAL_CLAIM_GROUP_NAME {
			continue
		}
		groupAditionalProperties = claim.AdditionalProperties
		break
	}

	emitAsRoles := slices.Contains(groupAditionalProperties, msgraph.OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_EMIT_AS_ROLES)

	switch {
	case slices.Contains(groupAditionalProperties, msgraph.OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_SAM_ACCOUNT_NAME):
		return emitAsRoles, func(g *msgraph.Group) string {
			if g.OnPremisesSamAccountName == nil {
				return getGroupID(g)
			}
			return *g.OnPremisesSamAccountName
		}
	case slices.Contains(groupAditionalProperties, msgraph.OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_DNS_DOMAIN_AND_SAM_ACCOUNT_NAME):
		return emitAsRoles, func(g *msgraph.Group) string {
			if g.OnPremisesSamAccountName == nil || g.OnPremisesDomainName == nil {
				return getGroupID(g)
			}
			return *g.OnPremisesDomainName + `\` + *g.OnPremisesSamAccountName
		}
	case slices.Contains(groupAditionalProperties, msgraph.OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_NETBIOS_DOMAIN_AND_SAM_ACCOUNT_NAME):
		return emitAsRoles, func(g *msgraph.Group) string {
			if g.OnPremisesSamAccountName == nil || g.OnPremisesNetBiosName == nil {
				return getGroupID(g)
			}
			return *g.OnPremisesNetBiosName + `\` + *g.OnPremisesSamAccountName
		}

	}
	return emitAsRoles, getGroupID
}

// groupFilterMatcher matches group based on the
// configured group filters.
func groupFilterMatcher(
	filters filter.Filters,
) func(g *msgraph.Group) bool {
	return func(g *msgraph.Group) bool {
		return filter.Matches(filters, filter.MatchParam{
			ID:   *g.ID,
			Name: *g.DisplayName,
		})
	}
}

// groupLocalMatcher matches group with an existing
// Entra ID Access List in Teleport.
func groupLocalMatcher(
	inACLMap map[string]*accessListWithMembers,
) func(g *msgraph.Group) bool {
	return func(g *msgraph.Group) bool {
		aclName := accessListName(*g.DisplayName, *g.ID)
		_, ok := inACLMap[aclName]
		return ok
	}
}
