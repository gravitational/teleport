package okta

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/utils/retryutils"
	accesslistsvc "github.com/gravitational/teleport/e/lib/accesslist"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// defaultAccessListSyncInterval defines a 30 minute default time between running
	// the access list synchronizer.
	defaultAccessListSyncInterval = 30 * time.Minute

	// Wait 5 minutes for the first loop in hopes that the synchronizer has run.
	accessListSyncFirstDuration = 5 * time.Minute

	// ReviewerSuffix is the string to append to the end of a role to indicate it's for reviews.
	ReviewerSuffix = "-reviewer"

	// Importer
	ImporterName = "okta-importer"
)

// accessListSyncConfig is the configuration for the access list synchronizer.
type accessListSyncConfig struct {
	// Log is the logger for the access list synchronizer.
	Log *logrus.Entry

	// Clock is the clock to use for the access list synchronizer.
	Clock clockwork.Clock

	// ClusterName is the name of the cluster.
	ClusterName string

	// Client is the okta client.
	Client OktaClient

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
	AppsGetter func() map[string]types.Application

	// GroupsGetter is the function to get groups to import.
	GroupsGetter func() map[string]types.UserGroup

	// AppFilters are regexes to filter the access list sync. Only app names that match one of these filters
	// will be added.
	AppFilters []*regexp.Regexp

	// GroupFilters are regexes to filter the access list sync. Only groups names that match one of these
	// filters will be added.
	GroupFilters []*regexp.Regexp

	// SynchronizerSuccess is expected to be true if the Okta synchronizer has completed at least
	// once successfully.
	SynchronizerSuccess *atomic.Bool

	// SynchronizingMu is a mutex that is held while synchronization is happening.
	SynchronizingMu *sync.RWMutex

	// StopChannel is the stop channel.
	StopChannel chan struct{}
}

func (a *accessListSyncConfig) CheckAndSetDefaults() error {
	if a.Log == nil {
		a.Log = logrus.WithField(teleport.ComponentKey, eteleport.ComponentOkta)
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

	if a.SyncInterval == 0 {
		a.SyncInterval = defaultAccessListSyncInterval
	}

	if a.AppsGetter == nil {
		return trace.BadParameter("missing apps getter")
	}

	if a.GroupsGetter == nil {
		return trace.BadParameter("missing groups getter")
	}

	if a.SynchronizerSuccess == nil {
		return trace.BadParameter("missing synchronizer success")
	}

	if a.SynchronizingMu == nil {
		return trace.BadParameter("missing synchronizing wait mutex")
	}

	if a.StopChannel == nil {
		return trace.BadParameter("missing stop channel")
	}

	return nil
}

// accessListSync will import user permissions information from Okta and create access lists
// and other resources to reflect them.
type accessListSync struct {
	log *logrus.Entry

	clock       clockwork.Clock
	clusterName string

	// client is the Okta client so that the importer can query the Okta API.
	client OktaClient

	// owners is the default owners for access lists.
	owners []accesslist.Owner

	orgURL string

	syncInterval time.Duration

	emitter     apievents.Emitter
	access      services.Access
	accessLists services.AccessLists

	// getters to retrieve the synchronized apps and groups to import.
	appsGetter   func() map[string]types.Application
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

	synchronizerSuccess *atomic.Bool
	synchronizingMu     *sync.RWMutex
	stopCh              chan struct{}
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
		log:                 cfg.Log,
		clock:               cfg.Clock,
		clusterName:         cfg.ClusterName,
		client:              cfg.Client,
		owners:              owners,
		emitter:             cfg.Emitter,
		access:              cfg.Access,
		accessLists:         cfg.AccessLists,
		orgURL:              cfg.OrgURL,
		syncInterval:        cfg.SyncInterval,
		appsGetter:          cfg.AppsGetter,
		groupsGetter:        cfg.GroupsGetter,
		appFilters:          cfg.AppFilters,
		groupFilters:        cfg.GroupFilters,
		synchronizerSuccess: cfg.SynchronizerSuccess,
		synchronizingMu:     cfg.SynchronizingMu,
		stopCh:              cfg.StopChannel,
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
		Log:      a.log,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	a.accessListMemberReconciler, err = accesslistsvc.NewMemberReconciler(
		accesslistsvc.MemberReconcilerConfig{
			AccessListMembers: a.accessLists,
			Matcher:           MatchByLabels[*accesslist.AccessListMember](a.orgURL),
			Log:               a.log,
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
		Log:      a.log,
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
	newMembers := a.newImportAccessListMembers.Clone()

	for key, existing := range existingMembers {
		if new, ok := newMembers[key]; ok {
			if !existing.Spec.Expires.IsZero() {
				new.Spec.Expires = existing.Spec.Expires
			}

			if existing.Spec.Reason != "" {
				new.Spec.Reason = existing.Spec.Reason
			}
		}
	}

	a.log.Infof("Reconciling %d new memberships against %d old memberships",
		len(newMembers), len(existingMembers))
	memberErr := a.accessListMemberReconciler.Reconcile(ctx, newMembers, existingMembers)

	roleErr := a.roleReconciler.Reconcile(ctx)

	return trace.NewAggregate(alErr, memberErr, roleErr)
}

// startSync will start the access list synchronizer.
func (a *accessListSync) startSync(ctx context.Context) {
	a.log.Info("Starting Okta access list synchronizer")

	// Let's make sure that any existing access lists are reflected in the Okta requester role.
	if err := a.refreshCurrentImports(ctx); err != nil {
		a.log.Error("Unable to refresh current imports")
	} else {
		if err := a.addRolesToOktaRequester(ctx); err != nil {
			a.log.Error("Unable to update Okta requester role")
		}
	}

	jitter := retryutils.NewSeventhJitter()
	timer := a.clock.NewTimer(jitter(accessListSyncFirstDuration))
	defer timer.Stop()

	for {
		select {
		case <-timer.Chan():
		case <-a.stopCh:
			return
		case <-ctx.Done():
			return
		}

		// Block if we're actively synchronizing.
		if a.synchronizerSuccess.Load() {
			a.log.Info("Synchronizing access lists from Okta.")
			if err := a.importOktaNativeAssignmentsAsAccessLists(ctx); err != nil {
				a.log.WithError(err).Error("error importing Okta native assignments")
			}

			if err := a.addRolesToOktaRequester(ctx); err != nil {
				a.log.Error("Unable to update Okta requester role")
			}
		} else {
			a.log.Info("Okta synchronizer has not yet completed successfully")
		}

		timer.Reset(jitter(a.syncInterval))
	}
}

// refreshCurrentImports will seed the current import maps with what's currently reflected
// in the backend.
func (a *accessListSync) refreshCurrentImports(ctx context.Context) error {
	// Make sure we only refresh imports that we should be reconciling.
	match := MatchByLabels[types.Resource](a.orgURL)

	// Get new matching access lists.
	accessLists := map[string]*accesslist.AccessList{}
	var nextToken string
	for {
		var page []*accesslist.AccessList
		var err error
		page, nextToken, err = a.accessLists.ListAccessLists(ctx, 0 /* page size */, nextToken)
		if err != nil {
			// If we get access denied here, it's possible that there are no current imports. We'll
			// Try to keep going and synchronize.
			if trace.IsAccessDenied(err) {
				break
			}
			return trace.Wrap(err)
		}

		for _, accessList := range page {
			if match(accessList) {
				accessLists[accessList.GetName()] = accessList
			}
		}

		if nextToken == "" {
			break
		}
	}

	// Using those access lists, get the related access list members.
	accessListMembers := map[string]*accesslist.AccessListMember{}
	for _, accessList := range accessLists {
		for {
			var page []*accesslist.AccessListMember
			var err error
			page, nextToken, err = a.accessLists.ListAccessListMembers(ctx, accessList.GetName(), 0 /* page size */, nextToken)
			if err != nil {
				return trace.Wrap(err)
			}

			for _, member := range page {
				if match(member) {
					accessListMembers[memberMapKey(member)] = member
				}
			}

			if nextToken == "" {
				break
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
	a.importAccessLists.Set(accessLists)
	a.importAccessListMembers.Set(accessListMembers)
	a.importRoles.Set(roles)

	return nil
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
	a.synchronizingMu.RLock()
	apps := a.appsGetter()
	groups := a.groupsGetter()
	a.synchronizingMu.RUnlock()

	// Reset apps/groups counters
	a.appsImported.Store(0)
	a.groupsImported.Store(0)

	// Refresh the current imports from what's known in the backend and clear the new imports.
	a.log.Info("Refreshing current imports")
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

	userNameToIDMapping, err := a.client.listUsers(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	// We need the opposite relation for this sync.
	userMapping := make(map[string]string, len(userNameToIDMapping))
	for k, v := range userNameToIDMapping {
		userMapping[string(v)] = string(k)
	}

	appMapping := map[string]types.Application{}
	for _, app := range apps {
		appMapping[app.GetName()] = app
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Import all apps.
	go func() {
		defer wg.Done()
		a.importApps(ctx, importAppsParams{
			apps:        apps,
			userMapping: userMapping,
			importCh:    importCh,
		})
	}()

	// Import all groups.
	go func() {
		defer wg.Done()
		a.importGroups(ctx, importGroupsParams{
			groups:      groups,
			appMapping:  appMapping,
			userMapping: userMapping,
			importCh:    importCh,
		})
	}()

	// Wait for the importing to finish and for all the existing metadata to be processed.
	wg.Wait()
	close(importCh)

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
				a.log.WithError(err).Error("error converting metadata to resources")
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
		a.log.WithError(err).Warn("Unable to emit audit event")
	}
}

// importResourceMetadata is the information necessary to create import resources based on Okta assignments.
type importResourceMetadata struct {
	name            string
	title           string
	description     string
	roleAppLabels   map[string][]string
	roleGroupLabels map[string][]string
	members         []string
}

var errNoAssignments = errors.New("no assignments")

type importAppsParams struct {
	apps        map[string]types.Application
	userMapping map[string]string
	importCh    chan importResourceMetadata
}

func (a *accessListSync) importApps(ctx context.Context, params importAppsParams) {
	appIDProcessed := map[string]struct{}{}
	for _, app := range params.apps {
		log := a.log.WithFields(logrus.Fields{
			"app_name": app.GetName(),
		})

		appID, ok := app.GetLabel(eteleport.OktaAppIDLabel)
		if !ok {
			log.Debug("application has no internal app ID label")
			continue
		}

		log = log.WithFields(logrus.Fields{
			"app_id": appID,
		})

		// the Okta app name will be used to match against filters.
		oktaAppName, ok := app.GetLabel(types.OktaAppNameLabel)
		if !ok {
			log.Debug("application has no internal Okta app name label")
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
				log.Debug("application doesn't match filter, skipping")
				continue
			}
		}

		if _, ok := appIDProcessed[appID]; ok {
			log.Debug("application ID was already processed")
			continue
		}

		irMetadata, err := a.appToImportResources(ctx, appID, app, params.userMapping)
		if err != nil && !errors.Is(err, errNoAssignments) {
			log.WithError(err).Error("error importing application")
			continue
		}

		appIDProcessed[appID] = struct{}{}

		if len(irMetadata.members) > 0 {
			log.Info("Processing application")
			params.importCh <- irMetadata
			a.appsImported.Add(1)
		} else {
			log.Info("Application has no assignments, skipping")
		}
	}
}

func (a *accessListSync) appToImportResources(ctx context.Context, appID string, app types.Application, userMapping map[string]string) (importResourceMetadata, error) {
	title, ok := app.GetLabel(types.OktaAppNameLabel)
	if !ok {
		a.log.WithField("app_id", appID).Debug("application ID has no app name to use as a title")
	}

	assignments, err := a.client.getAppAssignments(ctx, oktaAppID(appID))
	if err != nil {
		return importResourceMetadata{}, trace.Wrap(err)
	}

	// If there are no assignments, it's not an error, but return a nil labelsAndMembers object.
	numAssignments := len(assignments)
	if numAssignments == 0 {
		return importResourceMetadata{}, trace.Wrap(errNoAssignments)
	}

	members := make([]string, 0, numAssignments)
	for _, assignment := range assignments {
		// Add a member to the Okta App synced access list only if an Okta user has UserScope (the user has an individual Okta App assessment type).
		// We do not want to add users with GroupScope to App synced access list because this will cause redundancy were the user will be assigned as
		// member to both App and Group synced access list and will introduce duplicate access paths.
		if assignment.scope == userScope {
			if user, ok := userMapping[assignment.userID]; ok {
				members = append(members, user)
			}
		}
	}

	return importResourceMetadata{
		name:        app.GetName(),
		title:       title,
		description: "imported access list for Okta application",
		roleAppLabels: map[string][]string{
			eteleport.OktaAppIDLabel: {appID},
		},
		members: members,
	}, nil
}

type importGroupsParams struct {
	groups      map[string]types.UserGroup
	appMapping  map[string]types.Application
	userMapping map[string]string
	importCh    chan importResourceMetadata
}

func (a *accessListSync) importGroups(ctx context.Context, params importGroupsParams) {
	groupIDProcessed := map[string]struct{}{}
	for _, group := range params.groups {
		log := a.log.WithField("group_name", group.GetName())

		groupID, ok := group.GetLabel(eteleport.OktaGroupIDLabel)
		if !ok {
			a.log.WithField("group_name", group.GetName()).Debug("group has no internal group ID label")
			continue
		}

		log = log.WithField("group_id", groupID)

		if _, ok := groupIDProcessed[groupID]; ok {
			log.Debug("group ID was already processed")
			continue
		}

		// the Okta group name will be used to match against filters.
		oktaGroupName, ok := group.GetLabel(types.OktaGroupNameLabel)
		if !ok {
			log.Debug("application has no internal Okta group name label")
		}

		log = log.WithField("okta_group_name", oktaGroupName)

		if len(a.groupFilters) > 0 {
			matchFound := false
			for _, filter := range a.groupFilters {
				if filter.MatchString(oktaGroupName) {
					matchFound = true
					break
				}
			}

			if !matchFound {
				log.Debug("group doesn't match filter, skipping")
				continue
			}
		}

		irMetadata, err := a.groupToImportResources(ctx, groupID, group, params.appMapping, params.userMapping)
		if err != nil {
			log.Error("error importing group")
			continue
		}

		groupIDProcessed[groupID] = struct{}{}

		params.importCh <- irMetadata
		a.groupsImported.Add(1)
	}
}

func (a *accessListSync) groupToImportResources(ctx context.Context, groupID string, group types.UserGroup, appMapping map[string]types.Application, userMapping map[string]string) (importResourceMetadata, error) {
	title, ok := group.GetLabel(types.OktaGroupNameLabel)
	if !ok {
		return importResourceMetadata{}, trace.BadParameter("group ID %s has no group name to use as a title", groupID)
	}

	description, _ := group.GetLabel(types.OktaGroupDescriptionLabel)

	assignments, err := a.client.getGroupAssignments(ctx, oktaGroupID(groupID))
	if err != nil {
		return importResourceMetadata{}, trace.Wrap(err)
	}

	members := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		if user, ok := userMapping[string(assignment)]; ok {
			members = append(members, user)
		}
	}

	appLabels := map[string][]string{}
	for _, appName := range group.GetApplications() {
		app, ok := appMapping[appName]
		if !ok {
			a.log.Errorf("Unable to find app %s as part of group %s", appName, group.GetName())
			continue
		}

		oktaAppID, ok := app.GetLabel(eteleport.OktaAppIDLabel)
		if !ok {
			a.log.Errorf("Unable to find Okta App ID for app %s", appName)
			continue
		}

		appLabels[eteleport.OktaAppIDLabel] = append(appLabels[eteleport.OktaAppIDLabel], oktaAppID)
	}

	return importResourceMetadata{
		name:          groupID,
		title:         title,
		description:   description,
		roleAppLabels: appLabels,
		roleGroupLabels: map[string][]string{
			eteleport.OktaGroupIDLabel: {groupID},
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

	reviewerRoleName := irMetadata.name + ReviewerSuffix

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
			Roles: []string{irMetadata.name},
		},
		Owners: owners,
	})
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	members := make([]*accesslist.AccessListMember, 0, len(irMetadata.members))
	for _, memberName := range irMetadata.members {
		member, err := accesslist.NewAccessListMember(header.Metadata{
			Name:   memberName,
			Labels: labels,
		}, accesslist.AccessListMemberSpec{
			AccessList: accessList.GetName(),
			Name:       memberName,
			Joined:     a.clock.Now(),
			AddedBy:    ImporterName,
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

	role, err := types.NewRole(irMetadata.name, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Namespaces:  []string{apidefaults.Namespace},
			Rules:       rules,
			AppLabels:   toLabels(irMetadata.roleAppLabels),
			GroupLabels: toLabels(irMetadata.roleGroupLabels),
		},
	})
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	role.SetStaticLabels(labels)

	reviewerRole, err := types.NewRole(reviewerRoleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			ReviewRequests: &types.AccessReviewConditions{
				Roles: []string{role.GetName()},
			},
		},
	})
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	reviewerRole.SetStaticLabels(labels)

	return accessList, members, []types.Role{role, reviewerRole}, nil
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
			for roleName := range importRoles {
				if strings.HasSuffix(roleName, ReviewerSuffix) {
					continue
				}

				roles = append(roles, roleName)
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
