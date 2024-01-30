package okta

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/utils/retryutils"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// defaultAccessListSyncInterval defines a 30 minute default time between running
	// the access list synchronizer.
	defaultAccessListSyncInterval = 30 * time.Minute

	// Wait 5 minutes for the first loop in hopes that the synchronizer has run.
	accessListSyncFirstDuration = 5 * time.Minute

	// reviewerSuffix is the string to append to the end of a role to indicate it's for reviews.
	reviewerSuffix = "-reviewer"
)

// accessListSyncConfig is the configuration for the access list synchronizer.
type accessListSyncConfig struct {
	// Log is the logger for the access list synchronizer.
	Log *logrus.Entry

	// Clock is the clock to use for the access list synchronizer.
	Clock clockwork.Clock

	// Client is the okta client.
	Client OktaClient

	// Owners is the default owners for access lists.
	Owners []string

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
		a.Log = logrus.WithField(trace.Component, eteleport.ComponentOkta)
	}

	if a.Clock == nil {
		a.Clock = clockwork.NewRealClock()
	}

	if a.Client == nil {
		return trace.BadParameter("missing client")
	}

	if len(a.Owners) == 0 {
		return trace.BadParameter("missing owners")
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

	clock clockwork.Clock

	// client is the Okta client so that the importer can query the Okta API.
	client OktaClient

	// owners is the default owners for access lists.
	owners []accesslist.Owner

	orgURL string

	syncInterval time.Duration

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
	importAccessListsMu sync.Mutex
	importAccessLists   map[string]*accesslist.AccessList

	// newImportAccessLists is the mapping of access lists imported from Okta, not yet synchronized
	// to the import reconciler.
	newImportAccessListsMu sync.Mutex
	newImportAccessLists   map[string]*accesslist.AccessList

	// accessListMemberReconciler will sync imported access list members to the backend.
	accessListMemberReconciler *services.Reconciler[*accesslist.AccessListMember]

	// importAccessListMembers is the current mapping of imported access list members.
	importAccessListMembersMu sync.Mutex
	importAccessListMembers   map[string]*accesslist.AccessListMember

	// newAccessListMembers is the mapping of roles imported from Okta, not yet synchronized
	// to the import reconciler.
	newImportAccessListMembersMu sync.Mutex
	newImportAccessListMembers   map[string]*accesslist.AccessListMember

	// roleReconciler will sync imported roles to the backend.
	roleReconciler *services.Reconciler[types.Role]

	// importRoles is the current mapping of imported roles.
	importRolesMu sync.Mutex
	importRoles   map[string]types.Role

	// newImportRoles is the mapping of roles imported from Okta, not yet synchronized
	// to the import reconciler.
	newImportRolesMu sync.Mutex
	newImportRoles   map[string]types.Role

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
		log:                        cfg.Log,
		clock:                      cfg.Clock,
		client:                     cfg.Client,
		owners:                     owners,
		access:                     cfg.Access,
		accessLists:                cfg.AccessLists,
		orgURL:                     cfg.OrgURL,
		syncInterval:               cfg.SyncInterval,
		appsGetter:                 cfg.AppsGetter,
		groupsGetter:               cfg.GroupsGetter,
		appFilters:                 cfg.AppFilters,
		groupFilters:               cfg.GroupFilters,
		importAccessLists:          map[string]*accesslist.AccessList{},
		newImportAccessLists:       map[string]*accesslist.AccessList{},
		importRoles:                map[string]types.Role{},
		newImportRoles:             map[string]types.Role{},
		importAccessListMembers:    map[string]*accesslist.AccessListMember{},
		newImportAccessListMembers: map[string]*accesslist.AccessListMember{},
		synchronizerSuccess:        cfg.SynchronizerSuccess,
		synchronizingMu:            cfg.SynchronizingMu,
		stopCh:                     cfg.StopChannel,
	}

	// Create the reconcilers we need.
	var err error
	a.accessListReconciler, err = services.NewReconciler(services.ReconcilerConfig[*accesslist.AccessList]{
		Matcher:             matchByLabels[*accesslist.AccessList](a.orgURL),
		GetCurrentResources: a.getImportAccessLists,
		GetNewResources:     a.getNewImportAccessLists,
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

	a.accessListMemberReconciler, err = services.NewReconciler(services.ReconcilerConfig[*accesslist.AccessListMember]{
		Matcher:             matchByLabels[*accesslist.AccessListMember](a.orgURL),
		GetCurrentResources: a.getImportAccessListMembers,
		GetNewResources:     a.getNewImportAccessListMembers,
		OnCreate:            a.onUpsertAccessListMember,
		OnUpdate: func(ctx context.Context, member, _ *accesslist.AccessListMember) error {
			return a.onUpsertAccessListMember(ctx, member)
		},
		OnDelete: a.onDeleteAccessListMember,
		Log:      a.log,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	a.roleReconciler, err = services.NewReconciler(services.ReconcilerConfig[types.Role]{
		Matcher:             matchByLabels[types.Role](a.orgURL),
		GetCurrentResources: a.getImportRoles,
		GetNewResources:     a.getNewImportRoles,
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
	memberErr := a.accessListMemberReconciler.Reconcile(ctx)
	roleErr := a.roleReconciler.Reconcile(ctx)

	return trace.NewAggregate(alErr, memberErr, roleErr)
}

// startSync will start the access list synchronizer.
func (a *accessListSync) startSync(ctx context.Context) {
	a.log.Info("Starting Okta access list synchronizer")

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
	match := matchByLabels[types.Resource](a.orgURL)

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
	a.importAccessListsMu.Lock()
	a.importAccessLists = accessLists
	a.importAccessListsMu.Unlock()

	a.importAccessListMembersMu.Lock()
	a.importAccessListMembers = accessListMembers
	a.importAccessListMembersMu.Unlock()

	a.importRolesMu.Lock()
	a.importRoles = roles
	a.importRolesMu.Unlock()

	return nil
}

// clearNewImports will clear all of the existing newImport maps.
func (a *accessListSync) clearNewImports() {
	a.newImportAccessListsMu.Lock()
	clear(a.newImportAccessLists)
	a.newImportAccessListsMu.Unlock()

	a.newImportAccessListMembersMu.Lock()
	clear(a.newImportAccessListMembers)
	a.newImportAccessListMembersMu.Unlock()

	a.newImportRolesMu.Lock()
	clear(a.newImportRoles)
	a.newImportRolesMu.Unlock()
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

	// Refresh the current imports from what's known in the backend and clear the new imports.
	a.log.Info("Refreshing current imports")
	if err := a.refreshCurrentImports(ctx); err != nil {
		return trace.Wrap(err)
	}
	a.clearNewImports()

	importCh := make(chan *importResourceMetadata, 100)

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
		userMapping[v] = k
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Import all apps.
	go func() {
		defer wg.Done()
		a.importApps(ctx, apps, userMapping, importCh)
	}()

	// Import all groups.
	go func() {
		defer wg.Done()
		a.importGroups(ctx, groups, userMapping, importCh)
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
	return trace.Wrap(a.reconcileAll(ctx))
}

// convertAccessListMetadata will take access list metadata and turn it into import resources. On return, it will
// close the processDone channel.
func (a *accessListSync) convertAccessListMetadata(ctx context.Context, importCh chan *importResourceMetadata) {
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
			a.newImportAccessListsMu.Lock()
			a.newImportAccessLists[accessList.GetName()] = accessList
			a.newImportAccessListsMu.Unlock()

			a.newImportAccessListMembersMu.Lock()
			for _, member := range members {
				a.newImportAccessListMembers[memberMapKey(member)] = member
			}
			a.newImportAccessListMembersMu.Unlock()

			a.newImportRolesMu.Lock()
			for _, role := range roles {
				a.newImportRoles[role.GetName()] = role
			}
			a.newImportRolesMu.Unlock()

		case <-a.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// importResourceMetadata is the information necessary to create import resources based on Okta assignments.
type importResourceMetadata struct {
	name            string
	title           string
	description     string
	roleAppLabels   map[string]string
	roleGroupLabels map[string]string
	members         []string
}

var errNoAssignments = errors.New("no assignments")

func (a *accessListSync) importApps(ctx context.Context, apps map[string]types.Application, userMapping map[string]string, importCh chan *importResourceMetadata) {
	appIDProcessed := map[string]struct{}{}
	for _, app := range apps {
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

		irMetadata, err := a.appToImportResources(ctx, appID, app, userMapping)
		if err != nil && !errors.Is(err, errNoAssignments) {
			log.WithError(err).Error("error importing application")
			continue
		}

		appIDProcessed[appID] = struct{}{}

		if irMetadata != nil {
			log.Info("Processing application")
			importCh <- irMetadata
		} else {
			log.Info("Application has no assignments, skipping")
		}
	}
}

func (a *accessListSync) appToImportResources(ctx context.Context, appID string, app types.Application, userMapping map[string]string) (*importResourceMetadata, error) {
	title, ok := app.GetLabel(types.OktaAppNameLabel)
	if !ok {
		a.log.WithField("app_id", appID).Debug("application ID has no app name to use as a title")
	}

	assignments, err := a.client.getAppAssignments(ctx, appID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If there are no assignments, it's not an error, but return a nil labelsAndMembers object.
	numAssignments := len(assignments)
	if numAssignments == 0 {
		return nil, trace.Wrap(errNoAssignments)
	}

	members := make([]string, 0, numAssignments)
	for _, assignment := range assignments {
		if user, ok := userMapping[assignment]; ok {
			members = append(members, user)
		}
	}

	return &importResourceMetadata{
		name:        appID,
		title:       title,
		description: "imported access list for Okta application",
		roleAppLabels: map[string]string{
			eteleport.OktaAppIDLabel: appID,
		},
		members: members,
	}, nil
}

func (a *accessListSync) importGroups(ctx context.Context, groups map[string]types.UserGroup, userMapping map[string]string, importCh chan *importResourceMetadata) {
	groupIDProcessed := map[string]struct{}{}
	for _, group := range groups {
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

		irMetadata, err := a.groupToImportResources(ctx, groupID, group, userMapping)
		if err != nil && !errors.Is(err, errNoAssignments) {
			log.Error("error importing group")
			continue
		}

		groupIDProcessed[groupID] = struct{}{}

		if irMetadata != nil {
			log.Info("Processing group")
			importCh <- irMetadata
		} else {
			log.Info("Group has no assignments, skipping")
		}
	}
}

func (a *accessListSync) groupToImportResources(ctx context.Context, groupID string, group types.ResourceWithLabels, userMapping map[string]string) (*importResourceMetadata, error) {
	title, ok := group.GetLabel(types.OktaGroupNameLabel)
	if !ok {
		return nil, trace.BadParameter("group ID %s has no group name to use as a title", groupID)
	}

	description, _ := group.GetLabel(types.OktaGroupDescriptionLabel)

	assignments, err := a.client.getGroupAssignments(ctx, groupID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	numAssignments := len(assignments)
	if numAssignments == 0 {
		return nil, trace.Wrap(errNoAssignments)
	}

	members := make([]string, 0, numAssignments)
	for _, assignment := range assignments {
		if user, ok := userMapping[assignment]; ok {
			members = append(members, user)
		}
	}

	return &importResourceMetadata{
		name:        groupID,
		title:       title,
		description: description,
		roleGroupLabels: map[string]string{
			eteleport.OktaGroupIDLabel: groupID,
		},
		members: members,
	}, nil
}

func (a *accessListSync) getImportAccessLists() map[string]*accesslist.AccessList {
	a.importAccessListsMu.Lock()
	defer a.importAccessListsMu.Unlock()

	copyMap := map[string]*accesslist.AccessList{}
	maps.Copy(copyMap, a.importAccessLists)
	return copyMap
}

func (a *accessListSync) getNewImportAccessLists() map[string]*accesslist.AccessList {
	a.newImportAccessListsMu.Lock()
	defer a.newImportAccessListsMu.Unlock()

	copyMap := map[string]*accesslist.AccessList{}
	maps.Copy(copyMap, a.newImportAccessLists)
	return copyMap
}

func (a *accessListSync) getImportAccessListMembers() map[string]*accesslist.AccessListMember {
	a.importAccessListMembersMu.Lock()
	defer a.importAccessListMembersMu.Unlock()

	copyMap := map[string]*accesslist.AccessListMember{}
	maps.Copy(copyMap, a.importAccessListMembers)
	return copyMap
}

func (a *accessListSync) getNewImportAccessListMembers() map[string]*accesslist.AccessListMember {
	a.newImportAccessListMembersMu.Lock()
	defer a.newImportAccessListMembersMu.Unlock()

	copyMap := map[string]*accesslist.AccessListMember{}
	maps.Copy(copyMap, a.newImportAccessListMembers)
	return copyMap
}

func (a *accessListSync) getImportRoles() map[string]types.Role {
	a.importRolesMu.Lock()
	defer a.importRolesMu.Unlock()

	copyMap := map[string]types.Role{}
	maps.Copy(copyMap, a.importRoles)
	return copyMap
}

func (a *accessListSync) getNewImportRoles() map[string]types.Role {
	a.newImportRolesMu.Lock()
	defer a.newImportRolesMu.Unlock()

	copyMap := map[string]types.Role{}
	maps.Copy(copyMap, a.newImportRoles)
	return copyMap
}

// metadataToImportResources will convert import resource metadata from Okta and convert it into import resources.
func (a *accessListSync) metadataToImportResources(irMetadata *importResourceMetadata) (*accesslist.AccessList, []*accesslist.AccessListMember, []types.Role, error) {
	labels := map[string]string{
		types.OriginLabel:                  types.OriginOkta,
		types.TeleportInternalResourceType: types.SystemResource,
		eteleport.OktaOrgURLLabel:          a.orgURL,
	}

	// If there's an existing access list, preserve the owners set.
	owners := a.owners
	a.importAccessListsMu.Lock()
	if oldAccessList, ok := a.importAccessLists[irMetadata.name]; ok {
		owners = oldAccessList.GetOwners()
	}
	a.importAccessListsMu.Unlock()

	reviewerRoleName := irMetadata.name + reviewerSuffix

	// Create an access list for the resources.
	accessList, err := accesslist.NewAccessList(header.Metadata{
		Name:   irMetadata.name,
		Labels: labels,
	}, accesslist.Spec{
		Title: irMetadata.title,
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
			AddedBy:    "okta-importer",
		})
		if err != nil {
			return nil, nil, nil, trace.Wrap(err)
		}
		members = append(members, member)
	}

	role, err := types.NewRole(irMetadata.name, types.RoleSpecV6{
		Allow: types.RoleConditions{
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

	a.importAccessListsMu.Lock()
	a.importAccessLists[accessList.GetName()] = accessList
	a.importAccessListsMu.Unlock()
	return nil
}

// onDeleteAccessList will delete an access list.
func (a *accessListSync) onDeleteAccessList(ctx context.Context, accessList *accesslist.AccessList) error {
	// If the access list is already missing from the backend, we'll proceed.
	if err := a.accessLists.DeleteAccessList(ctx, accessList.GetName()); err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	a.importAccessListsMu.Lock()
	delete(a.importAccessLists, accessList.GetName())
	a.importAccessListsMu.Unlock()
	return nil
}

// onUpsertAccessListMember will create or modify an access list member.
func (a *accessListSync) onUpsertAccessListMember(ctx context.Context, member *accesslist.AccessListMember) error {
	if _, err := a.accessLists.UpsertAccessListMember(ctx, member); err != nil {
		return trace.Wrap(err)
	}

	a.importAccessListMembersMu.Lock()
	a.importAccessListMembers[memberMapKey(member)] = member
	a.importAccessListMembersMu.Unlock()
	return nil
}

// onDeleteAccessListMember will delete an access list member.
func (a *accessListSync) onDeleteAccessListMember(ctx context.Context, member *accesslist.AccessListMember) error {
	// If an access list is deleted in the access list reconciler, it can remove all of the associated access list members.
	// If we're attempting to delete something that can't be found, then we'll consider that a success.
	if err := a.accessLists.DeleteAccessListMember(ctx, member.Spec.AccessList, member.GetName()); err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	a.importAccessListMembersMu.Lock()
	delete(a.importAccessListMembers, memberMapKey(member))
	a.importAccessListMembersMu.Unlock()
	return nil
}

// onUpsertRole will create or modify an access list member.
func (a *accessListSync) onUpsertRole(ctx context.Context, role types.Role) error {
	if _, err := a.access.UpsertRole(ctx, role); err != nil {
		return trace.Wrap(err)
	}

	a.importRolesMu.Lock()
	a.importRoles[role.GetName()] = role
	a.importRolesMu.Unlock()
	return nil
}

// onDeleteRole will delete an access list member.
func (a *accessListSync) onDeleteRole(ctx context.Context, role types.Role) error {
	// If we're trying to delete a role that no longer exists, we'll consider that a success.
	if err := a.access.DeleteRole(ctx, role.GetName()); err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	a.importRolesMu.Lock()
	delete(a.importRoles, role.GetName())
	a.importRolesMu.Unlock()
	return nil
}

// toLabels converts a map of strings to a types.Labels resource.
func toLabels(m map[string]string) types.Labels {
	var labels types.Labels
	if m != nil {
		labels = types.Labels{}
		for k, v := range m {
			labels[k] = []string{v}
		}
	}
	return labels
}

// matchByLabels will match a resource based on the labels.
func matchByLabels[T types.Resource](expectedOrgURL string) func(T) bool {
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
