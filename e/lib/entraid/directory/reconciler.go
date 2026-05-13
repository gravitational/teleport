package directory

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/e/lib/mdmsync"
	"github.com/gravitational/teleport/lib/msgraph/models"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/plugins/filter"
	"github.com/gravitational/teleport/lib/services"
)

// Reconciler uses the Microsoft Graph API
// to synchronize Entra ID users and groups into the Teleport cluster
// as users and access lists.
type Reconciler struct {
	clock           clockwork.Clock
	logger          *slog.Logger
	metricsRegistry prometheus.Registerer
	metrics         *directoryMetrics
	graphClient     *graphClient
	accessPoint     accessPoint

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
	// accessListOwnersSource specifies source of the owners for the
	// Access Lists created for the Entra ID groups.
	// If Entra ID is configured as the source, but there are zero
	// owners configured for the group, default owners will be used
	// as Access List owners.
	accessListOwnersSource types.EntraIDAccessListOwnersSource
	// errSkippedResources holds collection of errors related to user,
	// group or group member import. These errors should be reported
	// after the reconciler returns.
	errSkippedResources errSkippedResources
}

type errSkippedResources struct {
	users, groups, groupMembers []error
}

// Config specifies dependencies and parameters for instantiating Service.
type Config struct {
	Clock  clockwork.Clock
	Logger *slog.Logger
	// MetricsRegistry is used to register metrics. When nil, metrics are not registered.
	MetricsRegistry *metrics.Registry
	// GraphClient is the instantiated Microsoft Graph API client.
	GraphClient GraphClient
	AccessPoint accessPoint

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
	// accessListOwnersSource specifies source of the owners for the
	// Access Lists created for the Entra ID groups.
	// If Entra ID is configured as the source, but there are zero
	// owners configured for the group, default owners will be used
	// as Access List owners.
	AccessListOwnersSource types.EntraIDAccessListOwnersSource
}

// Validate ensures that required values are set.
func (cfg *Config) Validate() error {
	if cfg.Clock == nil {
		return trace.BadParameter("Clock is required")
	}
	if cfg.Logger == nil {
		return trace.BadParameter("Logger is required")
	}
	if cfg.GraphClient == nil {
		return trace.BadParameter("GraphClient is required")
	}
	if cfg.AccessPoint == nil {
		return trace.BadParameter("AccessPoint is required")
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

	if cfg.EntraAppID == "" {
		return trace.BadParameter("EntraAppID is required")
	}

	for i, v := range cfg.DefaultOwners {
		if !v.IsMembershipKindUser() {
			continue
		}
		// Set IneligibleStatus to ELIGIBLE for user owners.
		// This optimizes the reconciler by skipping ineligibility checks,
		// since Entra ID access lists do not have owner eligibility requirements.
		cfg.DefaultOwners[i].IneligibleStatus = accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String()
	}

	if cfg.MetricsRegistry == nil {
		cfg.MetricsRegistry = metrics.NoopRegistry()
	}

	if cfg.AccessListOwnersSource == types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_UNSPECIFIED {
		cfg.AccessListOwnersSource = types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_PLUGIN
	}

	return nil
}

// New returns a new Reconciler using the given config.
func New(cfg Config) (*Reconciler, error) {
	if err := cfg.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}

	metrics, err := newMetrics(cfg.MetricsRegistry)
	if err != nil {
		return nil, trace.Wrap(err, "creating metrics")
	}
	if err := metrics.register(cfg.MetricsRegistry); err != nil {
		cfg.Logger.ErrorContext(context.Background(), "Failed to register metrics.", "err", err)
		// killing the process because we cannot expose metrics would cause more harm
		// than continuing.
	}

	return &Reconciler{
		clock:                  cfg.Clock,
		logger:                 cfg.Logger,
		metricsRegistry:        cfg.MetricsRegistry,
		graphClient:            newGraphClient(cfg.GraphClient, cfg.Logger),
		accessPoint:            cfg.AccessPoint,
		defaultOwners:          cfg.DefaultOwners,
		tenantID:               cfg.TenantID,
		ssoConnectorID:         cfg.SSOConnectorID,
		entraAppID:             cfg.EntraAppID,
		groupsFilter:           cfg.GroupsFilter,
		metrics:                metrics,
		accessListOwnersSource: cfg.AccessListOwnersSource,
	}, nil
}

// Reconcile does a one-time reconciliation of users and access lists
// from Entra ID to Teleport.
func (r *Reconciler) Reconcile(ctx context.Context, _ mdmsync.SyncMode) (err error) {
	defer func() {
		r.metrics.reconciliationCount.With(prometheus.Labels{
			metricLabelResult: metricLabelResultFromError(err),
		}).Inc()
	}()

	r.errSkippedResources = errSkippedResources{}
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
	teleportAccessListsWithMembersMap, err := listTeleportAccessListsWithMembers(ctx, r.accessPoint)
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
	entraGroupsResp, err := r.graphClient.listEntraGroups(ctx, entraGroupMatcher, r.accessListOwnersSource)
	if err != nil {
		return trace.Wrap(err)
	}
	r.errSkippedResources.groups = append(r.errSkippedResources.groups, entraGroupsResp.errSkippedGroups...)
	took = r.clock.Since(start)
	r.logger.DebugContext(ctx, "Finished listing Entra ID groups", "took", took.String())
	r.metrics.reconciliationDuration.With(prometheus.Labels{
		metricLabelSection: "read_entra_groups",
	}).Observe(took.Seconds())
	r.metrics.discoveredEntraGroups.Set(float64(len(entraGroupsResp.groups)))

	start = r.clock.Now()
	groupMembersMap, err := r.graphClient.listEntraGroupsMembers(ctx, entraGroupsResp.groups)
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
	usersByEntraID, err := r.reconcileUsers(ctx, entraGroupsResp.groups, groupMembersMap)
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
		entraGroupsResp.groups, groupMembersMap,
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

	if err := r.syncErrors(); err != nil {
		// Clear errors as they do not have any
		// purpose beyond this point.
		r.errSkippedResources = errSkippedResources{}
		return trace.Wrap(err)
	}

	return nil
}

func (r *Reconciler) reconcileUsers(ctx context.Context,
	groupsMap map[string]*models.Group,
	groupMembersMap map[string][]models.GroupMember,
) (map[entraUniqueID]types.User, error) {
	app, err := r.getApplication(ctx, r.entraAppID)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get Entra ID application")
	}
	emitAsRoles, groupNameBuilder := getGroupNameBuilderFunc(app)

	userMemberships := buildUserMemberships(groupsMap, groupMembersMap, groupNameBuilder)
	entraUsersResp, err := r.graphClient.listEntraUsers(ctx, userMemberships, r.tenantID, r.ssoConnectorID, emitAsRoles)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	r.errSkippedResources.users = append(r.errSkippedResources.users, entraUsersResp.errSkippedUsers...)
	r.metrics.discoveredEntraUsers.Set(float64(len(entraUsersResp.users)))

	connector, err := r.accessPoint.GetSAMLConnector(ctx, r.ssoConnectorID, false /* withSecrets */)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get SAML connector")
	}

	for _, entraUser := range entraUsersResp.users {
		_, roles := services.TraitsToRoles(connector.GetTraitMappings(), entraUser.GetTraits())
		entraUser.SetRoles(roles)
	}

	teleportUsers, err := listTeleportUsers(ctx, r.accessPoint, r.ssoConnectorID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, src := range teleportUsers {
		if dst, ok := entraUsersResp.users[src.GetName()]; ok {
			preserveUserMetadata(dst, src)
		}
	}

	usersByEntraID := map[entraUniqueID]types.User{}
	for _, u := range entraUsersResp.users {
		id, ok := u.GetLabel(types.EntraUniqueIDLabel)
		if !ok {
			return nil, trace.BadParameter("user %v missing Entra ID unique ID label", u.GetName())
		}
		usersByEntraID[entraUniqueID(id)] = u
	}
	var conflictingUsers []string
	backend, err := services.NewReconciler(services.ReconcilerConfig[types.User]{
		Matcher:             matchByLabel[types.User],
		CompareResources:    func(u1, u2 types.User) int { return services.EqualFromBool(u1.IsEqual(u2)) },
		GetCurrentResources: func() map[string]types.User { return teleportUsers },
		GetNewResources:     func() map[string]types.User { return entraUsersResp.users },
		OnCreate: func(ctx context.Context, u types.User) error {
			_, err := r.accessPoint.CreateUser(ctx, u)
			// if Entra user clashes with a local user, do not overwrite
			if trace.IsAlreadyExists(err) {
				conflictingUsers = append(conflictingUsers, u.GetName())

				// Delete from the lookup map, since the Teleport user by this name is not an Entra user.
				delete(usersByEntraID, entraUniqueID(u.GetMetadata().Labels[types.EntraUniqueIDLabel]))

				return nil
			}
			return trace.Wrap(err)
		},
		OnUpdate: func(ctx context.Context, incoming types.User, existing types.User) error {
			_, err := r.accessPoint.UpdateUser(ctx, incoming)
			return trace.Wrap(err)
		},
		OnDelete: func(ctx context.Context, u types.User) error {
			return trace.Wrap(r.accessPoint.DeleteUser(ctx, u.GetName()))
		},
		Metrics:            r.metrics.userReconcilerMetrics,
		AllowOriginChanges: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := backend.Reconcile(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	r.importedUsers = len(usersByEntraID)

	var errConflict error
	if len(conflictingUsers) != 0 {
		errConflict = trace.AlreadyExists(`existing user account found which was not created by this Microsoft Entra ID `+
			`integration, account and group sync will be skipped for these user(s): %s`, strings.Join(conflictingUsers, ", "))
		r.errSkippedResources.users = append(r.errSkippedResources.users, errConflict)
	}

	return usersByEntraID, nil
}

func (r *Reconciler) reconcileAccessLists(ctx context.Context,
	usersByEntraID map[entraUniqueID]types.User,
	groupsMap map[string]*models.Group,
	groupMembersMap map[string][]models.GroupMember,
	teleportAccessListsWithMembersMap map[string]*accessListWithMembers,
) error {
	aclOwnersCfg := aclOwnersConfig{
		defaultOwners: r.defaultOwners,
		source:        r.accessListOwnersSource,
	}
	g := entraGroups{
		groupsMap:       groupsMap,
		groupMembersMap: groupMembersMap,
	}
	entraAccessListWithMembersMap, errSkippedResources := g.toAccessListsWithMembers(ctx, r.tenantID, usersByEntraID, aclOwnersCfg)
	r.errSkippedResources.groups = slices.Concat(r.errSkippedResources.groups, errSkippedResources.groups)
	r.errSkippedResources.groupMembers = slices.Concat(r.errSkippedResources.groupMembers, errSkippedResources.groupMembers)

	// It's crucial to sort the members for the CompareResources func in the Reconciler.
	sortMembers(teleportAccessListsWithMembersMap)
	sortMembers(entraAccessListWithMembersMap)

	preserveFields(entraAccessListWithMembersMap, teleportAccessListsWithMembersMap)

	// If this is the first time import - there is not entraID access list in teleport
	// we can do a fast InsertAccessListCollection operation.
	// This significantly speeds up the first time import when there are thousands of groups
	// because we avoid per-access list upsert operations like locking
	// and fetching the existing resource from the backend.
	// The InsertAccessListCollection validates access list up front and due to fact the
	// collection is complete snapshot of all access lists and members from one source (EntraID)
	if len(teleportAccessListsWithMembersMap) == 0 && len(entraAccessListWithMembersMap) > 0 {
		r.logger.InfoContext(ctx, "EntraID initial access list import", "count", len(entraAccessListWithMembersMap))
		coll, err := toCollection(entraAccessListWithMembersMap)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := r.accessPoint.InsertAccessListCollection(ctx, coll); err != nil {
			return trace.Wrap(err)
		}
		r.logger.InfoContext(ctx, "EntraID initial access list import completed", "count", len(entraAccessListWithMembersMap))
		r.importedGroups = len(entraAccessListWithMembersMap)
		// No need to do further reconciliation. All access lists were inserted.
		return nil
	}

	var alsWithNestedMembers []*accessListWithMembers
	onUpsert := func(ctx context.Context, a *accessListWithMembers) error {
		hasNestedMember := slices.ContainsFunc(a.Members, func(m *accesslist.AccessListMember) bool {
			return m.Spec.MembershipKind == accesslist.MembershipKindList
		})

		// If access list has a nested member, don't upsert it with members, because some
		// of the member list may not be provisioned yet. It will be upserted with members
		// on a second pass when all access lists are already upserted.
		if hasNestedMember {
			if _, err := r.accessPoint.UpsertAccessList(ctx, a.AccessList); err != nil {
				return trace.Wrap(err)
			}
			alsWithNestedMembers = append(alsWithNestedMembers, a)
			return nil
		}
		_, _, err := r.accessPoint.UpsertAccessListWithMembers(ctx, a.AccessList, a.Members)
		return trace.Wrap(err)
	}

	alReconciler, err := services.NewReconciler(services.ReconcilerConfig[*accessListWithMembers]{
		CompareResources:    func(a, b *accessListWithMembers) int { return services.EqualFromBool(a.isEqual(b)) },
		Matcher:             func(a *accessListWithMembers) bool { return matchByLabel(a.AccessList) },
		GetCurrentResources: func() map[string]*accessListWithMembers { return teleportAccessListsWithMembersMap },
		GetNewResources:     func() map[string]*accessListWithMembers { return entraAccessListWithMembersMap },
		OnCreate: func(ctx context.Context, a *accessListWithMembers) error {
			return trace.Wrap(onUpsert(ctx, a))
		},
		OnUpdate: func(ctx context.Context, incoming, existing *accessListWithMembers) error {
			return trace.Wrap(onUpsert(ctx, incoming))
		},
		OnDelete: func(ctx context.Context, a *accessListWithMembers) error {
			// DeleteAccessList will also delete its members.
			err := r.accessPoint.DeleteAccessList(ctx, a.AccessList.GetName())
			return trace.Wrap(err)
		},
		Metrics: r.metrics.accessListReconcilerMetrics,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	var reconcileErrs []error

	if err := alReconciler.Reconcile(ctx); err != nil {
		reconcileErrs = append(reconcileErrs, trace.Wrap(err))
	}

	// Now all access lists are upserted, we can do a second pass and upsert all members for
	// access lists with nested members.
	// BTW, there is no need to do the same thing for potential nested owners as all owners are
	// overwritten with r.defaultOwners.
	var start time.Time
	for _, a := range alsWithNestedMembers {
		start = r.clock.Now()
		_, _, err := r.accessPoint.UpsertAccessListWithMembers(ctx, a.AccessList, a.Members)
		if err != nil {
			reconcileErrs = append(reconcileErrs, trace.Wrap(err))
		}
		r.metrics.reconciledNestedMemberDuration.Observe(r.clock.Since(start).Seconds())
		r.metrics.reconciledNestedMemberTotal.WithLabelValues(metricLabelResultFromError(err)).Inc()
	}

	if len(reconcileErrs) > 0 {
		return trace.NewAggregate(reconcileErrs...)
	}

	r.importedGroups = len(entraAccessListWithMembersMap)
	return nil
}

// ImportedUsers returns the total number of users imported as of the most recent reconciliation.
func (r *Reconciler) ImportedUsers() int {
	return r.importedUsers
}

// ImportedUsers returns the total number of groups imported as of the most recent reconciliation.
func (r *Reconciler) ImportedGroups() int {
	return r.importedGroups
}

func matchByLabel[T types.Resource](resource T) bool {
	origin, ok := resource.GetMetadata().Labels[types.OriginLabel]
	return ok && origin == types.OriginEntraID
}

func (r *Reconciler) getApplication(ctx context.Context, appID string) (*models.Application, error) {
	app, err := r.graphClient.GetApplication(ctx, appID)
	return app, trace.Wrap(err, "failed to get application")
}

func getGroupNameBuilderFunc(app *models.Application) (bool, func(*models.Group) string) {
	getGroupID := func(group *models.Group) string {
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
		if claim.Name == nil || *claim.Name != models.OPTIONAL_CLAIM_GROUP_NAME {
			continue
		}
		groupAditionalProperties = claim.AdditionalProperties
		break
	}

	emitAsRoles := slices.Contains(groupAditionalProperties, models.OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_EMIT_AS_ROLES)

	switch {
	case slices.Contains(groupAditionalProperties, models.OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_SAM_ACCOUNT_NAME):
		return emitAsRoles, func(g *models.Group) string {
			if g.OnPremisesSamAccountName == nil {
				return getGroupID(g)
			}
			return *g.OnPremisesSamAccountName
		}
	case slices.Contains(groupAditionalProperties, models.OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_DNS_DOMAIN_AND_SAM_ACCOUNT_NAME):
		return emitAsRoles, func(g *models.Group) string {
			if g.OnPremisesSamAccountName == nil || g.OnPremisesDomainName == nil {
				return getGroupID(g)
			}
			return *g.OnPremisesDomainName + `\` + *g.OnPremisesSamAccountName
		}
	case slices.Contains(groupAditionalProperties, models.OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_NETBIOS_DOMAIN_AND_SAM_ACCOUNT_NAME):
		return emitAsRoles, func(g *models.Group) string {
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
) func(g *models.Group) bool {
	return func(g *models.Group) bool {
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
) func(g *models.Group) bool {
	return func(g *models.Group) bool {
		aclName := accessListName(*g.DisplayName, *g.ID)
		_, ok := inACLMap[aclName]
		return ok
	}
}

func (r *Reconciler) syncErrors() error {
	errUsers := errors.Join(r.errSkippedResources.users...)
	errG := errors.Join(r.errSkippedResources.groups...)
	errGm := errors.Join(r.errSkippedResources.groupMembers...)
	return errors.Join(errUsers, errG, errGm)
}
