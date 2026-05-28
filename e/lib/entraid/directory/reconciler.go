package directory

import (
	"context"
	"errors"
	"log/slog"
	"slices"
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
	// deltaSyncEnabled indicates if delta is enabled or disabled.
	deltaSyncEnabled bool
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
	// DeltaSyncEnabled indicates if delta is enabled or disabled.
	DeltaSyncEnabled bool
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

	graphClient := newGraphClient(graphClientConfig{
		GraphClient:            cfg.GraphClient,
		deltaStore:             newDeltaStore(),
		accessListOwnersSource: cfg.AccessListOwnersSource,
		log:                    cfg.Logger,
	})

	return &Reconciler{
		clock:                  cfg.Clock,
		logger:                 cfg.Logger,
		metricsRegistry:        cfg.MetricsRegistry,
		graphClient:            graphClient,
		accessPoint:            cfg.AccessPoint,
		defaultOwners:          cfg.DefaultOwners,
		tenantID:               cfg.TenantID,
		ssoConnectorID:         cfg.SSOConnectorID,
		entraAppID:             cfg.EntraAppID,
		groupsFilter:           cfg.GroupsFilter,
		metrics:                metrics,
		accessListOwnersSource: cfg.AccessListOwnersSource,
		deltaSyncEnabled:       cfg.DeltaSyncEnabled,
	}, nil
}

// Result is the reconciliation result.
type Result struct {
	// ImportedUsers is the total number of users imported.
	ImportedUsers int
	// ImportedGroups is the total number of groups imported.
	ImportedGroups int
	// ErrSkippedResources is the [Reconciler.errSkippedResources]
	// error collected during sync.
	ErrSkippedResources error
}

// Reconcile does a one-time reconciliation of users and access lists
// from Entra ID to Teleport.
func (r *Reconciler) Reconcile(ctx context.Context, syncMode mdmsync.SyncMode) (result Result, err error) {
	defer func() {
		result.ImportedUsers = r.importedUsers
		result.ImportedGroups = r.importedGroups
		result.ErrSkippedResources = r.syncErrors()

		metricErr := err
		if syncMode == mdmsync.SyncModeFull && !r.deltaSyncEnabled {
			// TODO(sshah): Stop sending sync warnigns as error
			// once the plugin status and UI supports diffrentiating
			// between hard reconciler errors and skipped warnings.
			metricErr = errors.Join(err, result.ErrSkippedResources)
		}

		r.metrics.reconciliationCount.With(prometheus.Labels{
			metricLabelResult: metricLabelResultFromError(metricErr),
		}).Inc()
	}()

	r.errSkippedResources = errSkippedResources{}
	reconciliationStart := r.clock.Now()

	start := r.clock.Now()
	teleportAccessListsWithMembersMap, err := listTeleportAccessListsWithMembers(ctx, r.accessPoint)
	if err != nil {
		return result, trace.Wrap(err)
	}
	took := r.clock.Since(start)
	r.metrics.reconciliationDuration.With(prometheus.Labels{
		metricLabelSection: "read_list_backend",
	}).Observe(took.Seconds())

	teleportUsers, err := listTeleportUsers(ctx, r.accessPoint, r.ssoConnectorID)
	if err != nil {
		return result, trace.Wrap(err)
	}

	var errDeltaSetup error
	if syncMode == mdmsync.SyncModeFull && r.deltaSyncEnabled {
		// Full sync sets up the latest delta token which will be used
		// by the next delta sync. Delta token must be set up before
		// listing Entra directory so it can track any changes that
		// may happen while running the full sync.
		// Error is processed at the end to let the full sync continue.
		errDeltaSetup = r.graphClient.setupUserAndGroupDelta(ctx)
	}

	teleportState := teleportEntraDirectoryState{
		accessListsMap: teleportAccessListsWithMembersMap,
		usersMap:       teleportUsers,
	}

	start = r.clock.Now()
	entraGroupResp, err := r.getEntraGroupsAndMembers(ctx, syncMode, teleportState)
	r.errSkippedResources.groups = append(r.errSkippedResources.groups, entraGroupResp.errSkippedGroups...)
	if err != nil {
		return result, trace.Wrap(err)
	}
	r.logger.DebugContext(ctx,
		"Finished listing Entra ID groups and members",
		"sync_mode", syncMode,
		"took", r.clock.Since(start),
	)
	entraGroups := entraGroups{
		groupsMap:       entraGroupResp.groupsMap,
		groupMembersMap: entraGroupResp.groupMembersMap,
	}

	start = r.clock.Now()
	entraUsersResp, err := r.getEntraUsers(ctx, syncMode, teleportState.usersMap, entraGroups)
	r.errSkippedResources.users = append(r.errSkippedResources.users, entraUsersResp.errSkippedUsers...)
	if err != nil {
		return result, trace.Wrap(err)
	}

	r.logger.DebugContext(ctx,
		"Finished listing Entra ID users",
		"sync_mode", syncMode,
		"took", r.clock.Since(start),
		"limit", r.graphClient.graphClientLimit,
	)

	start = r.clock.Now()
	usersByEntraID, err := r.reconcileUsers(ctx, entraUsersResp.users, teleportState.usersMap)
	if err != nil {
		return result, trace.Wrap(err)
	}
	took = r.clock.Since(start)
	r.logger.DebugContext(ctx, "Finished reconciling Entra ID and Teleport users", "sync_mode", syncMode, "took", took.String())
	r.metrics.reconciliationDuration.With(prometheus.Labels{
		metricLabelSection: "reconcile_users",
	}).Observe(took.Seconds())

	start = r.clock.Now()
	aclOwnersCfg := aclOwnersConfig{
		defaultOwners: r.defaultOwners,
		source:        r.accessListOwnersSource,
	}
	entraAccessListsWithMembersMap, errSkipped := entraGroups.toAccessListsWithMembers(ctx, r.tenantID, usersByEntraID, aclOwnersCfg)
	r.errSkippedResources.groups = slices.Concat(r.errSkippedResources.groups, errSkipped.groups)
	r.errSkippedResources.groupMembers = slices.Concat(r.errSkippedResources.groupMembers, errSkipped.groupMembers)
	if err := r.reconcileAccessLists(ctx,
		entraAccessListsWithMembersMap,
		teleportAccessListsWithMembersMap,
	); err != nil {
		return result, trace.Wrap(err)
	}
	took = r.clock.Since(start)
	r.logger.DebugContext(ctx, "Finished reconciling Entra ID groups and Teleport Access Lists", "sync_mode", syncMode, "took", took.String())

	r.metrics.reconciliationDuration.With(prometheus.Labels{
		metricLabelSection: "reconcile_access_lists",
	}).Observe(took.Seconds())
	r.metrics.reconciliationDuration.With(prometheus.Labels{
		metricLabelSection: "total",
	}).Observe(r.clock.Since(reconciliationStart).Seconds())

	if errDeltaSetup != nil {
		// Returning error will force the next sync to be a full sync.
		r.logger.ErrorContext(ctx, "Latest delta token setup failed, delta sync will be skipped and the next sync will be a full sync", "error", errDeltaSetup)
		return result, trace.Wrap(errDeltaSetup)
	}

	return
}

func (r *Reconciler) getEntraGroupsAndMembers(
	ctx context.Context,
	syncMode mdmsync.SyncMode,
	teleportState teleportEntraDirectoryState,
) (listEntraGroupsAndMembersResponse, error) {
	var out listEntraGroupsAndMembersResponse
	entraGroupMatcher, err := r.buildGroupFilterMatcher(ctx, teleportState.accessListsMap)
	if err != nil {
		return out, trace.Wrap(err)
	}

	if syncMode == mdmsync.SyncModePartial {
		resp, err := r.graphClient.listEntraGroupsDelta(
			ctx,
			entraGroupMatcher,
			teleportState.accessListsMap,
			teleportState.usersMap,
		)

		return resp, trace.Wrap(err)
	}

	start := r.clock.Now()
	groupResp, err := r.graphClient.listEntraGroups(ctx, entraGroupMatcher, r.accessListOwnersSource)
	if err != nil {
		return out, trace.Wrap(err)
	}
	out.groupsMap = groupResp.groupsMap
	out.errSkippedGroups = groupResp.errSkippedGroups
	took := r.clock.Since(start)
	r.logger.DebugContext(ctx,
		"Finished listing Entra ID groups",
		"sync_mode", syncMode,
		"took", took,
	)
	r.metrics.reconciliationDuration.With(prometheus.Labels{
		metricLabelSection: "read_entra_groups",
	}).Observe(took.Seconds())
	r.metrics.discoveredEntraGroups.Set(float64(len(groupResp.groupsMap)))

	start = r.clock.Now()
	membersResp, err := r.graphClient.listEntraGroupsMembers(ctx, groupResp.groupsMap)
	if err != nil {
		return out, trace.Wrap(err)
	}
	out.groupMembersMap = membersResp
	took = r.clock.Since(start)
	r.logger.DebugContext(ctx,
		"Finished listing Entra ID group members",
		"sync_mode", syncMode,
		"took", took,
	)
	r.metrics.reconciliationDuration.With(prometheus.Labels{
		metricLabelSection: "read_entra_members",
	}).Observe(took.Seconds())
	r.metrics.discoveredEntraMemberships.Set(float64(len(membersResp)))

	return out, nil
}

func (r *Reconciler) getEntraUsers(
	ctx context.Context,
	syncMode mdmsync.SyncMode,
	teleportUsers map[string]types.User,
	entraGroups entraGroups,
) (listEntraUsersResponse, error) {

	var resp listEntraUsersResponse
	app, err := r.getApplication(ctx, r.entraAppID)
	if err != nil {
		return resp, trace.Wrap(err, "failed to get Entra ID application")
	}
	emitAsRoles, groupNameBuilder := getGroupNameBuilderFunc(app)
	connector, err := r.accessPoint.GetSAMLConnector(ctx, r.ssoConnectorID, false /* withSecrets */)
	if err != nil {
		return resp, trace.Wrap(err, "failed to get SAML connector")
	}

	userMemberships := buildUserMemberships(entraGroups, groupNameBuilder)
	cfg := entraUserSyncCfg{
		userConfig: userConfig{
			tenantID:       r.tenantID,
			ssoConnectorID: r.ssoConnectorID,
			emitAsRoles:    emitAsRoles,
			tms:            connector.GetTraitMappings(),
		},
		teleportUsers:    teleportUsers,
		usersMemberships: userMemberships,
	}

	if syncMode == mdmsync.SyncModePartial {
		resp, err := r.graphClient.listEntraUsersDelta(ctx, cfg)
		return resp, trace.Wrap(err, "failed processing user deltas")
	}

	resp, err = r.graphClient.listEntraUsers(ctx, cfg)
	r.metrics.discoveredEntraUsers.Set(float64(len(resp.users)))
	return resp, trace.Wrap(err)
}

func (r *Reconciler) reconcileUsers(ctx context.Context,
	entraUsers, teleportUsers map[string]types.User,
) (map[entraUniqueID]types.User, error) {

	usersByEntraID := map[entraUniqueID]types.User{}
	for _, u := range entraUsers {
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
		GetNewResources:     func() map[string]types.User { return entraUsers },
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

	if len(conflictingUsers) != 0 {
		r.errSkippedResources.users = append(r.errSkippedResources.users, errConflictingUsers(conflictingUsers))
	}

	return usersByEntraID, nil
}

func (r *Reconciler) reconcileAccessLists(ctx context.Context,
	entraAccessListWithMembersMap, teleportAccessListsWithMembersMap map[string]*accessListWithMembers,
) error {

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

func (r *Reconciler) buildGroupFilterMatcher(
	ctx context.Context,
	accessListMap map[string]*accessListWithMembers,
) (func(g *models.Group) bool, error) {
	useLocalGroupMatcher := false
	groupFilterMatcher := groupFilterMatcher(r.groupsFilter)
	if _, err := filter.New(r.groupsFilter); err != nil {
		switch {
		case errors.Is(err, filter.ErrUnknownFilter):
			// If the configured filter has an unknown filter type,
			// which may happen during cluster downgrade where an older cluster
			// may not understand the newer filter type, the reconciliation
			// should only apply to the already-synced groups.
			useLocalGroupMatcher = true
			r.logger.ErrorContext(ctx,
				"Unknown group filter encountered, filters will be skipped and reconciliation will be limited to the existing Entra ID groups",
				"error", err.Error(),
			)
		default:
			return nil, trace.Wrap(err)
		}
	}

	if useLocalGroupMatcher {
		groupFilterMatcher = groupLocalMatcher(accessListMap)
	}

	return groupFilterMatcher, nil
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

// teleportEntraDirectoryState holds Access Lists and users
// resources created from Entra ID groups and users.
type teleportEntraDirectoryState struct {
	accessListsMap map[string]*accessListWithMembers
	usersMap       map[string]types.User
}
