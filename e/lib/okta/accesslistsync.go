package okta

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/header"
	accesslistsvc "github.com/gravitational/teleport/e/lib/accesslist"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/e/lib/okta/common"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/accesslists"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/set"
)

const (

	// ReviewerSuffix is the string to append to the end of a role to indicate it's for reviews.
	ReviewerSuffix = "-reviewer"

	// Importer
	ImporterName = "okta-importer"
)

// accessListSyncConfig is the configuration for the access list synchronizer.
type accessListSyncConfig struct {
	// Log is the logger for the access list synchronizer.
	Logger *slog.Logger

	// Clock is the clock to use for the access list synchronizer.
	Clock clockwork.Clock

	// ClusterName is the name of the cluster.
	ClusterName string

	// Client is the okta client.
	Client oktaapi.Interface

	// Owners is the default owners for access lists.
	Owners []string

	// Emitter is events emitter, used to submit discrete events
	Emitter apievents.Emitter

	// Access is used by the access list sync service to interact users and roles.
	Access services.Access

	// AccessLists is used by the access list sync service to interact with access lists.
	AccessLists services.AccessLists

	// SyncInterval is the amount of time to wait between running the synchronizer.
	SyncInterval time.Duration

	// OrgURL is the Okta org URL.
	OrgURL string

	// AppsGetter is the function to get apps to import.
	AppsGetter func() map[string]types.AppServer

	// GroupsGetter is the function to get groups to import.
	GroupsGetter func() map[string]types.UserGroup

	// AppFilters are regexes to filter the access list sync. Only app names that match one of these filters
	// will be added.
	AppFilters []*regexp.Regexp

	// GroupFilters are regexes to filter the access list sync. Only groups names that match one of these
	// filters will be added.
	GroupFilters []*regexp.Regexp

	// StopChannel is the stop channel.
	StopChannel chan struct{}

	// ServiceStatus is the sink for detailed status information
	ServiceStatus serviceStatusUpdater

	// AssignmentsService is the service to use for assignments.
	// It MUST NOT be a cache, otherwise we will risk privileges escalation of short-term Access Requests turning into long-t erm.
	// https://github.com/gravitational/teleport-private/issues/1944.
	OktaAssignmentService common.OktaAssignmentService
}

func (a *accessListSyncConfig) CheckAndSetDefaults() error {
	if a.Logger == nil {
		a.Logger = slog.With(teleport.ComponentKey, eteleport.ComponentOkta)
	}

	if a.Clock == nil {
		a.Clock = clockwork.NewRealClock()
	}

	if a.ClusterName == "" {
		return trace.BadParameter("missing cluster name")
	}

	if a.Client == nil {
		return trace.BadParameter("missing client")
	}

	if len(a.Owners) == 0 {
		return trace.BadParameter("missing owners")
	}

	if a.Emitter == nil {
		return trace.BadParameter("missing emitter")
	}

	if a.Access == nil {
		return trace.BadParameter("missing access")
	}

	if a.AccessLists == nil {
		return trace.BadParameter("missing access lists")
	}

	if a.OrgURL == "" {
		return trace.BadParameter("missing org url")
	}

	if a.AppsGetter == nil {
		return trace.BadParameter("missing apps getter")
	}

	if a.GroupsGetter == nil {
		return trace.BadParameter("missing groups getter")
	}

	if a.StopChannel == nil {
		return trace.BadParameter("missing stop channel")
	}

	if a.ServiceStatus == nil {
		return trace.BadParameter("missing service status")
	}

	if a.OktaAssignmentService == nil {
		return trace.BadParameter("missing assignments service")
	}

	return nil
}

// accessListSync will import user permissions information from Okta and create access lists
// and other resources to reflect them.
type accessListSync struct {
	logger *slog.Logger

	clock       clockwork.Clock
	clusterName string

	// client is the Okta client so that the importer can query the Okta API.
	client oktaapi.Interface

	// owners is the default owners for access lists.
	owners []accesslist.Owner

	orgURL string

	syncInterval time.Duration

	emitter     apievents.Emitter
	access      services.Access
	accessLists services.AccessLists

	oktaUsers       map[userName]oktaUserID
	oktaUserMapping map[oktaUserID]userName
	oktaUsersMu     sync.Mutex

	// getters to retrieve the synchronized apps and groups to import.
	appsGetter   func() map[string]types.AppServer
	groupsGetter func() map[string]types.UserGroup

	// filters for groups and apps
	appFilters   []*regexp.Regexp
	groupFilters []*regexp.Regexp

	// accessListReconciler will sync imported access lists to the backend.
	accessListReconciler *services.Reconciler[*accesslist.AccessList]

	// importAccessLists is the current mapping of imported access lists.
	importAccessLists utils.SyncMap[string, *accesslist.AccessList]

	// newImportAccessLists is the mapping of access lists imported from Okta, not yet synchronized
	// to the import reconciler.
	newImportAccessLists utils.SyncMap[string, *accesslist.AccessList]

	// accessListMemberReconciler will sync imported access list members to the backend.
	accessListMemberReconciler *accesslistsvc.MemberReconciler

	// importAccessListMembers is the current mapping of imported access list members.
	importAccessListMembers utils.SyncMap[string, *accesslist.AccessListMember]

	// newAccessListMembers is the mapping of roles imported from Okta, not yet synchronized
	// to the import reconciler.
	newImportAccessListMembers utils.SyncMap[string, *accesslist.AccessListMember]

	// roleReconciler will sync imported roles to the backend.
	roleReconciler *services.Reconciler[types.Role]

	// importRoles is the current mapping of imported roles.
	importRoles utils.SyncMap[string, types.Role]

	// newImportRoles is the mapping of roles imported from Okta, not yet synchronized
	// to the import reconciler.
	newImportRoles utils.SyncMap[string, types.Role]

	// these are used to maintain app and group import stats per synchronization run.
	appsImported   atomic.Int32
	groupsImported atomic.Int32

	stopCh chan struct{}

	serviceStatus      serviceStatusUpdater
	assignmentsService common.OktaAssignmentService
}

// newAccessListSync will create a new access list synchronizer.
func newAccessListSync(cfg accessListSyncConfig) (*accessListSync, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	owners := make([]accesslist.Owner, len(cfg.Owners))
	for i, owner := range cfg.Owners {
		owners[i] = accesslist.Owner{
			Name:        owner,
			Description: "default owner from access list synchronizer",
		}
	}

	a := &accessListSync{
		logger:             cfg.Logger,
		clock:              cfg.Clock,
		clusterName:        cfg.ClusterName,
		client:             cfg.Client,
		owners:             owners,
		emitter:            cfg.Emitter,
		access:             cfg.Access,
		accessLists:        cfg.AccessLists,
		orgURL:             cfg.OrgURL,
		syncInterval:       cfg.SyncInterval,
		appsGetter:         cfg.AppsGetter,
		groupsGetter:       cfg.GroupsGetter,
		appFilters:         cfg.AppFilters,
		groupFilters:       cfg.GroupFilters,
		stopCh:             cfg.StopChannel,
		serviceStatus:      cfg.ServiceStatus,
		assignmentsService: cfg.OktaAssignmentService,
	}

	// Create the reconcilers we need.
	var err error
	a.accessListReconciler, err = services.NewReconciler(services.ReconcilerConfig[*accesslist.AccessList]{
		Matcher:             MatchByLabels[*accesslist.AccessList](a.orgURL),
		GetCurrentResources: a.importAccessLists.Clone,
		GetNewResources:     a.newImportAccessLists.Clone,
		OnCreate:            a.onUpsertAccessList,
		OnUpdate: func(ctx context.Context, accessList, _ *accesslist.AccessList) error {
			return a.onUpsertAccessList(ctx, accessList)
		},
		OnDelete: a.onDeleteAccessList,
		Logger:   a.logger.With("kind", types.KindAccessList),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	a.accessListMemberReconciler, err = accesslistsvc.NewMemberReconciler(
		accesslistsvc.MemberReconcilerConfig{
			AccessListMembers: a.accessLists,
			Matcher:           MatchByLabels[*accesslist.AccessListMember](a.orgURL),
			Logger:            a.logger,
			OnUpsert:          a.onUpsertAccessListMember,
			OnDelete:          a.onDeleteAccessListMember,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	a.roleReconciler, err = services.NewReconciler(services.ReconcilerConfig[types.Role]{
		Matcher:             MatchByLabels[types.Role](a.orgURL),
		GetCurrentResources: a.importRoles.Clone,
		GetNewResources:     a.newImportRoles.Clone,
		OnCreate:            a.onUpsertRole,
		OnUpdate: func(ctx context.Context, role, _ types.Role) error {
			return a.onUpsertRole(ctx, role)
		},
		OnDelete: a.onDeleteRole,
		Logger:   a.logger.With("kind", types.KindRole),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return a, nil
}

// reconcileAll will run reconcile on all reconcilers and return the aggregated error.
func (a *accessListSync) reconcileAll(ctx context.Context) error {
	alErr := a.accessListReconciler.Reconcile(ctx)

	existingMembers := a.importAccessListMembers.Clone()
	oktaMembers := a.newImportAccessListMembers.Clone()

	for key, existing := range existingMembers {
		if new, ok := oktaMembers[key]; ok {
			if !existing.Spec.Expires.IsZero() {
				new.Spec.Expires = existing.Spec.Expires
			}

			if existing.Spec.Reason != "" {
				new.Spec.Reason = existing.Spec.Reason
			}
		}
	}

	// Exclude Okta members who were assigned via an ongoing Access Request.
	// These temporary assignments should not be treated as long-term members
	// during the sync process, to avoid syncing back assignments created by short-term
	// access requests as long-term Access List (ACL) memberships in Teleport.
	//
	// Note: If the access request was promoted (`RequestState_PROMOTED`), then the Okta assignment
	// originating from the access request is not created. Instead, the Access Request promotion
	// results in a new Okta assignment based on the ACL membership.
	ongoingAccessRequestFilter := common.OngoingAssignmentsMembershipFilter{
		AssignmentsService: a.assignmentsService,
	}
	if err := ongoingAccessRequestFilter.Filter(ctx, oktaMembers, existingMembers); err != nil {
		return trace.Wrap(err)
	}
	a.logger.InfoContext(ctx, "Reconciling new memberships against existing memberships",
		"new_member_count", len(oktaMembers), "exiting_member_count", len(existingMembers))
	memberErr := a.accessListMemberReconciler.Reconcile(ctx, oktaMembers, existingMembers)

	roleErr := a.roleReconciler.Reconcile(ctx)

	return trace.NewAggregate(alErr, memberErr, roleErr)
}

// loadOktaUsers will load the Okta users and build a reverse mapping from user IDs to usernames,
// if not already loaded. Caller requires a lock on oktaUsersMu.
func (a *accessListSync) loadOktaUsers(ctx context.Context) error {
	// Check if Okta users are already loaded.
	if len(a.oktaUsers) > 0 && a.oktaUserMapping != nil {
		return nil
	}
	a.logger.InfoContext(ctx, "Loading Okta users")

	oktaUsers, err := a.client.ListUsers(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	a.oktaUsers = oktaUsers

	// Build reverse mapping from user IDs to usernames.
	a.oktaUserMapping = make(map[oktaUserID]userName, len(oktaUsers))
	for name, id := range oktaUsers {
		a.oktaUserMapping[id] = name
	}

	return nil
}

// clearLoadedOktaUsers will clear the loaded Okta users and user mapping. Caller requires a lock on oktaUsersMu.
func (a *accessListSync) clearLoadedOktaUsers() {
	a.oktaUsers = nil
	a.oktaUserMapping = nil
}

func (a *accessListSync) init(ctx context.Context) {
	// Let's make sure that any existing access lists are reflected in the Okta requester role.
	if err := a.refreshCurrentImports(ctx); err != nil {
		a.logger.ErrorContext(ctx, "Unable to refresh current imports", "error", err)
		return
	}
	if err := a.addRolesToOktaRequester(ctx); err != nil {
		a.logger.ErrorContext(ctx, "Unable to update Okta requester role")
	}
}

// sync should be called after [accessListSync.init] is called.
func (a *accessListSync) sync(ctx context.Context) {
	err := a.importOktaNativeAssignmentsAsAccessLists(ctx)

	a.serviceStatus.UpdateAccessListSync(ctx, a.clock.Now(),
		int(a.appsImported.Load()),
		int(a.groupsImported.Load()),
		err)

	if err != nil {
		a.logger.ErrorContext(ctx, "Error synchronizing assignments imported from Okta", "error", err)
	}
	if err := a.addRolesToOktaRequester(ctx); err != nil {
		a.logger.ErrorContext(ctx, "Error updating Okta requester role", "error", err)
	}
}

// refreshCurrentImports will seed the current import maps with what's currently reflected
// in the backend.
func (a *accessListSync) refreshCurrentImports(ctx context.Context) error {
	a.oktaUsersMu.Lock()
	defer a.oktaUsersMu.Unlock()

	// Load Okta users if not already loaded.
	if err := a.loadOktaUsers(ctx); err != nil {
		return trace.Wrap(err)
	}

	// Make sure we only refresh imports that we should be reconciling.
	match := MatchByLabels[types.Resource](a.orgURL)

	// Get all access lists.
	allAccessLists, err := a.accessLists.GetAccessLists(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Get new matching access lists.
	matchingAccessLists := make(map[string]*accesslist.AccessList)
	for _, accessList := range allAccessLists {
		if match(accessList) {
			matchingAccessLists[accessList.GetName()] = accessList
		}
	}

	// Using those access lists, get the related access list members.
	accessListMembers := map[string]*accesslist.AccessListMember{}
	for _, accessList := range matchingAccessLists {
		members, err := a.getAndProcessMembers(ctx, accessList.GetName())
		if err != nil {
			return trace.Wrap(err)
		}

		// Match and store the members after fetching all recursively
		for _, member := range members {
			if match(member) {
				accessListMembers[memberMapKey(member)] = member
			}
		}
	}

	// Get matching roles.
	allRoles, err := a.access.GetRoles(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	roles := map[string]types.Role{}
	for _, role := range allRoles {
		if match(role) {
			roles[role.GetName()] = role
		}
	}

	// Refresh the currently known resources.
	a.importAccessLists.Set(matchingAccessLists)
	a.importAccessListMembers.Set(accessListMembers)
	a.importRoles.Set(roles)

	return nil
}

func (a *accessListSync) getAndProcessMembers(ctx context.Context, accessListName string) ([]*accesslist.AccessListMember, error) {
	allMembers, err := accesslists.GetMembersFor(ctx, accessListName, a.accessLists)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Use a map to deduplicate members.
	memberMap := make(map[string]*accesslist.AccessListMember)
	for _, member := range allMembers {
		a.processMember(member, accessListName)
		key := memberMapKey(member)
		if _, exists := memberMap[key]; !exists {
			memberMap[key] = member
		}
	}
	members := make([]*accesslist.AccessListMember, 0, len(memberMap))
	for _, member := range memberMap {
		members = append(members, member)
	}

	return members, nil
}

// processMember sets Okta labels on the Okta-originated users and sets the Access List name to the
// root Access List if this is a member of a nested Access List.
func (a *accessListSync) processMember(member *accesslist.AccessListMember, accessListName string) {
	// Check if the member exists in Okta users
	if len(a.oktaUsers) > 0 {
		// If the member exists in oktaUsers and has no origin set, artificially set it.
		// This avoids upserting the member to the root access list on reconciliation.
		if _, ok := a.oktaUsers[userName(member.Spec.Name)]; ok {
			if _, labelOk := member.Metadata.Labels[types.OriginLabel]; !labelOk {
				if member.Metadata.Labels == nil {
					member.Metadata.Labels = map[string]string{}
				}
				member.Metadata.Labels[types.OriginLabel] = types.OriginOkta
				member.Metadata.Labels[eteleport.OktaOrgURLLabel] = a.orgURL
			}
		}
	}

	// If the member is from a nested list, set Spec.AccessList to the root list.
	// This avoids the member being added as a duplicate to the root list.
	if member.Spec.AccessList != accessListName {
		member.Spec.AccessList = accessListName
	}
}

// clearNewImports will clear all of the existing newImport maps.
func (a *accessListSync) clearNewImports() {
	a.newImportAccessLists.Clear()
	a.newImportAccessListMembers.Clear()
	a.newImportRoles.Clear()
}

// importOktaNativeAssignmentsAsAccessLists will look through each known Okta application and group, get their
// assignments, and then import them as access lists into Teleport while assigning users to those access
// lists within Teleport.
//
// Additionally, Okta import rules and roles will be created to support these access list, as well as
// introducing roles to review relevant resources.
func (a *accessListSync) importOktaNativeAssignmentsAsAccessLists(ctx context.Context) error {
	apps := a.appsGetter()
	groups := a.groupsGetter()

	// Reset apps/groups counters
	a.appsImported.Store(0)
	a.groupsImported.Store(0)

	// Refresh the current imports from what's known in the backend and clear the new imports.
	a.logger.InfoContext(ctx, "Refreshing current imports")
	if err := a.refreshCurrentImports(ctx); err != nil {
		return trace.Wrap(err)
	}
	a.clearNewImports()

	importCh := make(chan importResourceMetadata, 100)

	convertContext, cancel := context.WithCancel(ctx)

	// This process will take the resource metadata and convert it into actual resources.
	go func() {
		defer cancel()
		a.convertAccessListMetadata(convertContext, importCh)
	}()

	a.oktaUsersMu.Lock()
	defer a.oktaUsersMu.Unlock()

	// Load Okta users if not already loaded.
	if err := a.loadOktaUsers(ctx); err != nil {
		return trace.Wrap(err)
	}
	userMapping := a.oktaUserMapping

	appMapping := map[string]types.AppServer{}
	for _, app := range apps {
		appMapping[app.GetName()] = app
	}

	eg, groupCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		err := a.importApps(groupCtx, importAppsParams{
			apps:        apps,
			userMapping: userMapping,
			importCh:    importCh,
		})
		return trace.Wrap(err)
	})

	eg.Go(func() error {
		err := a.importGroups(groupCtx, importGroupsParams{
			groups:      groups,
			appMapping:  appMapping,
			userMapping: userMapping,
			importCh:    importCh,
		})
		return trace.Wrap(err)
	})

	// Wait for the importing to finish and for all the existing metadata to be processed.
	err := eg.Wait()
	close(importCh)
	if err != nil {
		a.logger.ErrorContext(ctx, "Access List import will be skipped due Okta API error", "error", err)
		a.emitAccessListSyncEvent(ctx, err)
		return trace.Wrap(err)
	}

	select {
	case <-convertContext.Done():
	case <-a.stopCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}

	// Now that we've rebuilt our maps, run the reconciler.
	reconcileErr := a.reconcileAll(ctx)

	a.emitAccessListSyncEvent(ctx, reconcileErr)

	// Clear the cached Okta users so we have an up-to-date list next cycle.
	a.clearLoadedOktaUsers()

	return trace.Wrap(reconcileErr)
}

// convertAccessListMetadata will take access list metadata and turn it into import resources. On return, it will
// close the processDone channel.
func (a *accessListSync) convertAccessListMetadata(ctx context.Context, importCh chan importResourceMetadata) {
	for {
		select {
		case alMetadata, ok := <-importCh:
			if !ok {
				return
			}

			accessList, members, roles, err := a.metadataToImportResources(alMetadata)
			if err != nil {
				a.logger.ErrorContext(ctx, "error converting metadata to resources", "error", err)
				continue
			}

			// Push all of the resources into the various new maps.
			a.newImportAccessLists.Store(accessList.GetName(), accessList)

			a.newImportAccessListMembers.Write(
				func(newMembers map[string]*accesslist.AccessListMember) {
					for _, member := range members {
						newMembers[memberMapKey(member)] = member
					}
				})

			a.newImportRoles.Write(
				func(newImportRoles map[string]types.Role) {
					for _, role := range roles {
						newImportRoles[role.GetName()] = role
					}
				})

		case <-a.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// emitAccessListSyncEvent will take in a reconciler error and emit an audit event for the synchronization
// that just occurred.
func (a *accessListSync) emitAccessListSyncEvent(ctx context.Context, reconcileErr error) {
	code := events.OktaAccessListSyncSuccessCode
	var errorMsg string
	success := true
	if reconcileErr != nil {
		code = events.OktaAccessListSyncFailureCode
		errorMsg = reconcileErr.Error()
		success = false
	}

	event := &apievents.OktaAccessListSync{
		Metadata: apievents.Metadata{
			Type:        events.OktaAccessListSyncEvent,
			Code:        code,
			ClusterName: a.clusterName,
		},
		Status: apievents.Status{
			Error:   errorMsg,
			Success: success,
		},
		NumAppFilters:   int32(len(a.appFilters)),
		NumGroupFilters: int32(len(a.groupFilters)),
		NumApps:         a.appsImported.Load(),
		NumGroups:       a.groupsImported.Load(),
	}

	event.NumRoles = int32(a.importRoles.Len())
	event.NumAccessLists = int32(a.importAccessLists.Len())
	event.NumAccessListMembers = int32(a.importAccessListMembers.Len())

	if err := a.emitter.EmitAuditEvent(ctx, event); err != nil {
		a.logger.WarnContext(ctx, "Unable to emit audit event", "error", err)
	}
}

// importResourceMetadata is the information necessary to create import resources based on Okta assignments.
type importResourceMetadata struct {
	name            string
	title           string
	description     string
	roleAppLabels   map[string][]string
	roleGroupLabels map[string][]string
	members         []userName
}

var errNoAssignments = errors.New("no assignments")

type importAppsParams struct {
	apps        map[string]types.AppServer
	userMapping map[oktaUserID]userName
	importCh    chan importResourceMetadata
}

func getAppID(a types.AppServer) (oktaAppID, bool) {
	id, ok := a.GetLabel(eteleport.OktaAppIDLabel)
	return oktaAppID(id), ok
}

func (a *accessListSync) importApps(ctx context.Context, params importAppsParams) error {
	appIDProcessed := set.New[oktaAppID]()
	// Sort the apps by name. This is to make the generated resource names predictable. It may
	// a single Okta application has multiple links. If so, there is a separate app passed here
	// in the params for each Okta app link, but the resources here (Access Lists, Access List
	// Members and Roles) are generated only for the first encountered app/link. Because the
	// app.GetName() is generated with a hash value impacted by the link name we make sure
	// always use the the same app/link for each Okta app here.
	// https://github.com/gravitational/teleport.e/pull/5934#discussion_r1929587258
	sortedApps := slices.SortedFunc(maps.Values(params.apps), func(a, b types.AppServer) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	for _, app := range sortedApps {
		log := a.logger.With("app_name", app.GetName())

		appID, ok := getAppID(app)
		if !ok {
			log.DebugContext(ctx, "application has no internal app ID label")
			continue
		}

		log = log.With("app_id", appID)

		// the Okta app name will be used to match against filters.
		oktaAppName, ok := app.GetLabel(types.OktaAppNameLabel)
		if !ok {
			log.DebugContext(ctx, "application has no internal Okta app name label")
		}

		if len(a.appFilters) > 0 {
			matchFound := false
			for _, filter := range a.appFilters {
				if filter.MatchString(oktaAppName) {
					matchFound = true
					break
				}
			}

			if !matchFound {
				log.DebugContext(ctx, "application doesn't match filter, skipping")
				continue
			}
		}

		if appIDProcessed.Contains(appID) {
			log.DebugContext(ctx, "application ID was already processed")
			continue
		}

		irMetadata, err := a.appToImportResources(ctx, appID, app, params.userMapping)
		if err != nil && !errors.Is(err, errNoAssignments) {
			log.ErrorContext(ctx, "error importing application", "error", err)
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
			continue
		}

		appIDProcessed.Add(appID)

		if len(irMetadata.members) > 0 {
			log.InfoContext(ctx, "Processing application")
			params.importCh <- irMetadata
			a.appsImported.Add(1)
		} else {
			log.InfoContext(ctx, "Application has no assignments, skipping")
		}
	}
	return nil
}

func (a *accessListSync) appToImportResources(ctx context.Context, appID oktaAppID, appServer types.AppServer, userMapping map[oktaUserID]userName) (importResourceMetadata, error) {
	title, ok := appServer.GetLabel(types.OktaAppNameLabel)
	if !ok {
		a.logger.DebugContext(ctx, "application ID has no app name to use as a title", "app_id", appID)
	}

	assignments, err := a.client.GetAppAssignments(ctx, appID)
	if err != nil {
		return importResourceMetadata{}, trace.Wrap(err)
	}

	// If there are no assignments, it's not an error, but return a nil labelsAndMembers object.
	numAssignments := len(assignments)
	if numAssignments == 0 {
		return importResourceMetadata{}, trace.Wrap(errNoAssignments)
	}

	members := make([]userName, 0, numAssignments)
	for _, assignment := range assignments {
		// Add a member to the Okta App synced access list only if an Okta user has UserScope (the user has an individual Okta App assessment type).
		// We do not want to add users with GroupScope to App synced access list because this will cause redundancy were the user will be assigned as
		// member to both App and Group synced access list and will introduce duplicate access paths.
		if assignment.Scope == oktaapi.UserScope {
			if user, ok := userMapping[oktaUserID(assignment.UserID)]; ok {
				members = append(members, user)
			}
		}
	}

	return importResourceMetadata{
		name:        appServer.GetName(),
		title:       title,
		description: "imported access list for Okta application",
		roleAppLabels: map[string][]string{
			eteleport.OktaAppIDLabel: {string(appID)},
		},
		members: members,
	}, nil
}

type importGroupsParams struct {
	groups      map[string]types.UserGroup
	appMapping  map[string]types.AppServer
	userMapping map[oktaUserID]userName
	importCh    chan importResourceMetadata
}

func getGroupID(g types.UserGroup) (oktaGroupID, bool) {
	groupID, ok := g.GetLabel(eteleport.OktaGroupIDLabel)
	return oktaGroupID(groupID), ok
}

func (a *accessListSync) importGroups(ctx context.Context, params importGroupsParams) error {
	groupIDProcessed := set.New[oktaGroupID]()
	for _, group := range params.groups {
		log := a.logger.With("group_name", group.GetName())

		groupID, ok := getGroupID(group)
		if !ok {
			log.DebugContext(ctx, "group has no internal group ID label")
			continue
		}

		log = log.With("group_id", groupID)

		if groupIDProcessed.Contains(groupID) {
			log.DebugContext(ctx, "group ID was already processed")
			continue
		}

		// the Okta group name will be used to match against filters.
		oktaGroupName, ok := group.GetLabel(types.OktaGroupNameLabel)
		if !ok {
			log.DebugContext(ctx, "application has no internal Okta group name label")
		}

		log = log.With("okta_group_name", oktaGroupName)

		if len(a.groupFilters) > 0 {
			matchFound := false
			for _, filter := range a.groupFilters {
				if filter.MatchString(oktaGroupName) {
					matchFound = true
					break
				}
			}

			if !matchFound {
				log.DebugContext(ctx, "group doesn't match filter, skipping")
				continue
			}
		}

		irMetadata, err := a.groupToImportResources(ctx, groupID, group, params.appMapping, params.userMapping)
		if err != nil {
			if !trace.IsNotFound(err) {
				log.ErrorContext(ctx, "Error importing group", "error", err)
				return trace.Wrap(err)
			}
			continue
		}

		groupIDProcessed.Add(groupID)

		params.importCh <- irMetadata
		a.groupsImported.Add(1)
	}
	return nil
}

func (a *accessListSync) groupToImportResources(ctx context.Context, groupID oktaGroupID, group types.UserGroup, appServerMapping map[string]types.AppServer, userMapping map[oktaUserID]userName) (importResourceMetadata, error) {
	title, ok := group.GetLabel(types.OktaGroupNameLabel)
	if !ok {
		return importResourceMetadata{}, trace.BadParameter("group ID %s has no group name to use as a title", groupID)
	}

	description, _ := group.GetLabel(types.OktaGroupDescriptionLabel)

	assignments, err := a.client.GetGroupAssignments(ctx, groupID)
	if err != nil {
		return importResourceMetadata{}, trace.Wrap(err)
	}

	members := make([]userName, 0, len(assignments))
	for _, assignment := range assignments {
		if user, ok := userMapping[assignment]; ok {
			members = append(members, user)
		}
	}

	appLabels := map[string][]string{}
	for _, appName := range group.GetApplications() {
		appServer, ok := appServerMapping[appName]
		if !ok {
			a.logger.ErrorContext(ctx, "Unable to find app as part of group", "app", appName, "group", group.GetName())
			continue
		}

		oktaAppID, ok := appServer.GetLabel(eteleport.OktaAppIDLabel)
		if !ok {
			a.logger.ErrorContext(ctx, "Unable to find Okta App ID for app", "app", appName)
			continue
		}

		appLabels[eteleport.OktaAppIDLabel] = append(appLabels[eteleport.OktaAppIDLabel], oktaAppID)
	}

	return importResourceMetadata{
		name:          string(groupID),
		title:         title,
		description:   description,
		roleAppLabels: appLabels,
		roleGroupLabels: map[string][]string{
			eteleport.OktaGroupIDLabel: {string(groupID)},
		},
		members: members,
	}, nil
}

// metadataToImportResources will convert import resource metadata from Okta and convert it into import resources.
func (a *accessListSync) metadataToImportResources(irMetadata importResourceMetadata) (*accesslist.AccessList, []*accesslist.AccessListMember, []types.Role, error) {
	labels := map[string]string{
		types.OriginLabel:                  types.OriginOkta,
		types.TeleportInternalResourceType: types.SystemResource,
		eteleport.OktaOrgURLLabel:          a.orgURL,
	}

	// If there's an existing access list, preserve the following fields:
	// - Owners
	// - Owner Requirements
	// - Member Requirements
	// - Audit
	owners := a.owners
	var membershipRequires, ownershipRequires accesslist.Requires
	var audit accesslist.Audit

	a.importAccessLists.Read(
		func(importAccessLists map[string]*accesslist.AccessList) {
			if oldAccessList, ok := importAccessLists[irMetadata.name]; ok {
				owners = oldAccessList.GetOwners()
				membershipRequires = oldAccessList.Spec.MembershipRequires
				ownershipRequires = oldAccessList.Spec.OwnershipRequires
				audit = oldAccessList.Spec.Audit
			}
		})

	accessRoleName := common.CreateOktaAccessRoleFriendlyName(irMetadata.title, irMetadata.name)
	reviewerRoleName := common.CreateOktaReviewerRoleFriendlyName(irMetadata.title, irMetadata.name)
	// Create an access list for the resources.
	accessList, err := accesslist.NewAccessList(header.Metadata{
		Name:   irMetadata.name,
		Labels: labels,
	}, accesslist.Spec{
		Title:              irMetadata.title,
		MembershipRequires: membershipRequires,
		OwnershipRequires:  ownershipRequires,
		Audit:              audit,
		OwnerGrants: accesslist.Grants{
			Roles: []string{reviewerRoleName},
		},
		Grants: accesslist.Grants{
			Roles: []string{accessRoleName},
		},
		Owners: owners,
	})
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	members := make([]*accesslist.AccessListMember, 0, len(irMetadata.members))
	for _, memberName := range irMetadata.members {
		member, err := accesslist.NewAccessListMember(header.Metadata{
			Name:   string(memberName),
			Labels: labels,
		}, accesslist.AccessListMemberSpec{
			AccessList:     accessList.GetName(),
			Name:           string(memberName),
			Joined:         a.clock.Now(),
			AddedBy:        ImporterName,
			MembershipKind: accesslist.MembershipKindUser,
		})
		if err != nil {
			return nil, nil, nil, trace.Wrap(err)
		}
		members = append(members, member)
	}

	var rules []types.Rule
	if len(irMetadata.roleGroupLabels) > 0 {
		rules = append(rules, types.NewRule(types.KindUserGroup, services.RO()))
	}
	// TODO(sshah): update role version to v8 in Teleport version v19.0.0.
	accessRole, err := types.NewRoleWithVersion(accessRoleName, types.V7, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules:       rules,
			AppLabels:   toLabels(irMetadata.roleAppLabels),
			GroupLabels: toLabels(irMetadata.roleGroupLabels),
		},
	})
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	accessRole.SetStaticLabels(labels)
	// TODO(sshah): update role version to v8 in Teleport version v19.0.0.
	reviewerRole, err := types.NewRoleWithVersion(reviewerRoleName, types.V7, types.RoleSpecV6{
		Allow: types.RoleConditions{
			ReviewRequests: &types.AccessReviewConditions{
				Roles: []string{accessRole.GetName()},
			},
		},
	})
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	labelsCpy := maps.Clone(labels)
	labelsCpy[eteleport.OktaACLReviewerRoleLabel] = "true"
	reviewerRole.SetStaticLabels(labelsCpy)

	return accessList, members, []types.Role{accessRole, reviewerRole}, nil
}

// onUpsertAccessList will create or modify an access list.
func (a *accessListSync) onUpsertAccessList(ctx context.Context, accessList *accesslist.AccessList) error {
	if _, err := a.accessLists.UpsertAccessList(ctx, accessList); err != nil {
		return trace.Wrap(err)
	}
	a.importAccessLists.Store(accessList.GetName(), accessList)
	return nil
}

// onDeleteAccessList will delete an access list.
func (a *accessListSync) onDeleteAccessList(ctx context.Context, accessList *accesslist.AccessList) error {
	// If the access list is already missing from the backend, we'll proceed.
	if err := a.accessLists.DeleteAccessList(ctx, accessList.GetName()); err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	a.importAccessLists.Delete(accessList.GetName())
	return nil
}

// onUpsertAccessListMember will create or modify an access list member.
func (a *accessListSync) onUpsertAccessListMember(ctx context.Context, member *accesslist.AccessListMember) error {
	a.importAccessListMembers.Store(memberMapKey(member), member)
	return nil
}

// onDeleteAccessListMember will delete an access list member.
func (a *accessListSync) onDeleteAccessListMember(ctx context.Context, member *accesslist.AccessListMember) error {
	a.importAccessListMembers.Delete(memberMapKey(member))
	return nil
}

// onUpsertRole will create or modify an access list member.
func (a *accessListSync) onUpsertRole(ctx context.Context, role types.Role) error {
	if _, err := a.access.UpsertRole(ctx, role); err != nil {
		return trace.Wrap(err)
	}

	a.importRoles.Store(role.GetName(), role)
	return nil
}

// onDeleteRole will delete an access list member.
func (a *accessListSync) onDeleteRole(ctx context.Context, role types.Role) error {
	// If we're trying to delete a role that no longer exists, we'll consider that a success.
	if err := a.access.DeleteRole(ctx, role.GetName()); err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	a.importRoles.Delete(role.GetName())
	return nil
}

// addRolesToOktaRequester will add non-reviewer roles to the Okta requester.
func (a *accessListSync) addRolesToOktaRequester(ctx context.Context) error {
	var roles []string

	a.importRoles.Read(
		func(importRoles map[string]types.Role) {
			for k, v := range importRoles {
				if _, ok := v.GetLabel(eteleport.OktaACLReviewerRoleLabel); ok {
					continue
				}
				roles = append(roles, k)
			}
		})

	oktaRequesterRole, err := a.access.GetRole(ctx, teleport.SystemOktaRequesterRoleName)
	if err != nil {
		return trace.Wrap(err)
	}

	oktaRequesterRole.SetSearchAsRoles(types.Allow, roles)
	_, err = a.access.UpsertRole(ctx, oktaRequesterRole)
	return trace.Wrap(err)
}

// toLabels converts a map of strings to a types.Labels resource.
func toLabels(m map[string][]string) types.Labels {
	var labels types.Labels
	if len(m) > 0 {
		labels = types.Labels{}
		for k, v := range m {
			labels[k] = v
		}
	}
	return labels
}

// MatchByLabels will match a resource based on the labels.
func MatchByLabels[T types.Resource](expectedOrgURL string) func(T) bool {
	return func(resource T) bool {
		origin, ok := resource.GetMetadata().Labels[types.OriginLabel]
		if !ok || origin != types.OriginOkta {
			return false
		}

		orgURL, ok := resource.GetMetadata().Labels[eteleport.OktaOrgURLLabel]
		if !ok {
			return false
		}

		return orgURL == expectedOrgURL
	}
}

// memberMapKey returns an identifier for members that will be unique in the reconciler.
func memberMapKey(member *accesslist.AccessListMember) string {
	return fmt.Sprintf("%s/%s", member.Spec.AccessList, member.GetName())
}
