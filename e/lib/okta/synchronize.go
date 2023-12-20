package okta

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/okta/okta-sdk-golang/v2/okta"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// emit synchronize events in batches of 100
	syncEventBatches = 100

	// we'll choose a 10 second jitter for the synchronization loop to avoid potential contention
	// with other Okta services (should we ever decide to support multiple services)
	syncJitter = 10 * time.Second

	// we will wait for this amount of time before synchronization starts if
	// this Okta service is not the leader.
	syncRetryAfterLeadershipFailure = time.Minute
)

// synchronizeLoop will synchronize Okta with the backend periodically until the
// process is terminated.
func (s *Service) synchronizeLoop(ctx context.Context) {
	if shouldStop := s.waitIfNotLeader(ctx); shouldStop {
		return
	}

	ticker, timeBetweenSyncs := s.setupSynchronizerTicker(ctx)
	defer ticker.Stop()

	s.log.Infof("Synchronizer started with a refresh period of %s.", timeBetweenSyncs)

Loop:
	for {
		// If the parent Okta service is not the leader, skip synchronizing.
		if s.leadershipAcquired.Load() {
			timeoutCtx, cancel := context.WithTimeout(ctx, timeBetweenSyncs)
			if err := s.synchronize(timeoutCtx); err != nil {
				s.log.Errorf("Error while synchronizing Okta resources with Teleport: %v", err)
				s.emitSyncError(ctx, err)
			}
			cancel()
		}

		select {
		case <-ticker.Chan():
		case <-s.stopCh:
			break Loop
		case <-ctx.Done():
			break Loop
		}

		timeBetweenSyncs = s.updateSynchronizerTicker(ctx, ticker, timeBetweenSyncs)
	}

	s.log.Infof("Synchronizer stopped.")

	s.syncStoppedChCloser.Do(func() { close(s.syncStoppedCh) })
}

// waitIfNotLeader will wait for this service to become the leader. It will return true if the service should stop.
func (s *Service) waitIfNotLeader(ctx context.Context) bool {
	// Don't start the loop until we acquire leadership.
	s.log.Infof("Waiting for leadership to be acquired before starting synchronizer.")
	waitForLeadershipTicker := s.clock.NewTicker(syncRetryAfterLeadershipFailure)
	defer waitForLeadershipTicker.Stop()
	for {
		if s.leadershipAcquired.Load() {
			break
		}

		select {
		case <-waitForLeadershipTicker.Chan():
		case <-s.stopCh:
			return true
		case <-ctx.Done():
			return true
		}
	}

	return false
}

// emitSyncError will emit a sync error event to the Teleport audit log.
func (s *Service) emitSyncError(ctx context.Context, err error) {
	event := &apievents.OktaSyncFailure{
		Metadata: apievents.Metadata{
			Type: events.OktaSyncFailureEvent,
			Code: events.OktaSyncFailureCode,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerID: s.hostID,
		},
		Status: apievents.Status{
			Success: false,
			Error:   err.Error(),
		},
	}

	if emitErr := s.emitter.EmitAuditEvent(ctx, event); emitErr != nil {
		s.log.WithError(emitErr).Warnf("Failed to emit Okta synchronization failure event: %v", event)
	}
}

// setupSynchronizerTicker creates the ticker for the synchronizer.
func (s *Service) setupSynchronizerTicker(ctx context.Context) (clockwork.Ticker, time.Duration) {
	timeBetweenSyncs := s.timeBetweenSyncs
	pref, err := s.accessPoint.GetAuthPreference(ctx)
	if err != nil {
		s.log.WithError(err).Errorf("Error getting auth preference during synchronization, using service level default of %s", timeBetweenSyncs)
	} else {
		if pref.GetOktaSyncPeriod() != 0 {
			timeBetweenSyncs = pref.GetOktaSyncPeriod()
		}
	}

	// Generate a random jitter between 0 and 10 seconds
	ticker := s.clock.NewTicker(s.timeBetweenSyncs + utils.RandomDuration(syncJitter))

	return ticker, timeBetweenSyncs
}

// updateSynchronizerTicker updates the ticker for the synchronizer if necessary.
func (s *Service) updateSynchronizerTicker(ctx context.Context, ticker clockwork.Ticker, timeBetweenSyncs time.Duration) time.Duration {
	pref, err := s.accessPoint.GetAuthPreference(ctx)
	if err != nil {
		s.log.WithError(err).Error("Error getting auth preference during synchronization, continuing anyway")
		return timeBetweenSyncs
	}

	if timeBetweenSyncs != pref.GetOktaSyncPeriod() {
		if pref.GetOktaSyncPeriod() == 0 {
			timeBetweenSyncs = s.timeBetweenSyncs
		} else {
			timeBetweenSyncs = pref.GetOktaSyncPeriod()
		}

		ticker.Reset(timeBetweenSyncs + utils.RandomDuration(syncJitter))
		s.log.Infof("Synchronizer refresh period updated to %s.", timeBetweenSyncs)
	}
	return timeBetweenSyncs
}

// synchronize will synchronize the Okta groups and applications with the backend.
func (s *Service) synchronize(ctx context.Context) error {
	if err := s.syncUsers(ctx); err != nil {
		return trace.Wrap(err)
	}

	if err := s.buildImportRuleMappings(ctx); err != nil {
		return trace.Wrap(err)
	}

	groupsToAppsMapping, err := s.synchronizeApplications(ctx)
	if err != nil {
		s.log.Warnf("Error when synchronizing applications, unable to sync groups: %v", err)

		// We need the groups to apps mapping in order to synchronize groups properly, so
		// we won't try to synchronize groups if we can't synchronize apps.
		return trace.Wrap(err)
	}

	if err := s.synchronizeGroups(ctx, groupsToAppsMapping); err != nil {
		s.log.Warnf("Error when synchronizing groups: %v", err)
		return trace.Wrap(err)
	}

	return nil
}

// synchronizeGroups will synchronize Okta groups with the backend.
func (s *Service) synchronizeGroups(ctx context.Context, groupsToAppsMapping userGroupsToApplications) error {
	newGroups := map[string]types.UserGroup{}
	err := s.client.iterateGroups(ctx, func(oktaGroup *okta.Group) error {
		s.log.Debugf("Processing Okta group %v", oktaGroup.Id)

		userGroup, err := s.oktaGroupToUserGroup(oktaGroup, groupsToAppsMapping[oktaGroup.Id])
		if err != nil {
			s.log.Debugf("Error converting Okta group: %v", err)
			return nil
		}

		newGroups[userGroup.GetName()] = userGroup

		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	s.newGroupsMu.Lock()
	s.newGroups = newGroups
	s.newGroupsMu.Unlock()

	s.groupsAdded = nil
	s.groupsUpdated = nil
	s.groupsDeleted = nil

	if err := s.groupsReconciler.Reconcile(ctx); err != nil {
		return trace.Wrap(err, "error during group reconciliation")
	}

	// If all of the group stats are 0, skip the emit. We only want to emit on changes.
	if s.groupsAdded == nil && s.groupsUpdated == nil && s.groupsDeleted == nil {
		return nil
	}

	s.emitSyncEventsInBatches(ctx, events.OktaGroupsUpdateEvent, events.OktaGroupsUpdateCode, s.groupsAdded, s.groupsUpdated, s.groupsDeleted)

	return nil
}

// userGroupsToApplications is a mapping of user groups to applications.
type userGroupsToApplications map[string][]string

// synchronizeApplications will synchronize Okta applications with the backend.
func (s *Service) synchronizeApplications(ctx context.Context) (userGroupsToApplications, error) {
	s.log.Debug("Synchronizing applications")

	groupsToAppsMapping := userGroupsToApplications{}
	newApps := map[string]*types.AppV3{}
	err := s.client.iterateApps(ctx, func(oktaApp okta.App) error {
		// This type assertion is necessary as okta.App, which is supplied by the Okta go SDK,
		// does not contain all of the information that we need to create a types.Application
		// object.
		var oktaApplication *okta.Application
		var ok bool
		if oktaApplication, ok = oktaApp.(*okta.Application); ok {
			s.log.Debugf("Processing Okta application %v", oktaApplication.Id)
		} else {
			s.log.Debugf("Unable to process Okta application of unknown type %T", oktaApp)
			return nil
		}

		groups, err := s.client.getAppGroups(ctx, oktaApplication.Id)
		if err != nil {
			s.log.Debugf("Error getting groups for applications: %v", err)
			return nil
		}

		apps, err := s.oktaAppToApp(oktaApplication, groups)
		if err != nil {
			s.log.Debugf("Error converting Okta app: %v", err)
			return nil
		}

		for _, app := range apps {
			newApps[app.GetName()] = app
			for _, group := range groups {
				groupsToAppsMapping[group] = append(groupsToAppsMapping[group], app.GetName())
			}
		}

		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.newAppsMu.Lock()
	s.newApps = newApps
	s.newAppsMu.Unlock()

	s.appsAdded = nil
	s.appsUpdated = nil
	s.appsDeleted = nil

	if err := s.appsReconciler.Reconcile(ctx); err != nil {
		return nil, trace.Wrap(err, "error during application reconciliation")
	}

	// If all of the app stats are 0, skip the emit. We only want to emit on changes.
	if s.appsAdded == nil && s.appsUpdated == nil && s.appsDeleted == nil {
		return groupsToAppsMapping, nil
	}

	s.emitSyncEventsInBatches(ctx, events.OktaApplicationsUpdateEvent, events.OktaApplicationsUpdateCode, s.appsAdded, s.appsUpdated, s.appsDeleted)

	return groupsToAppsMapping, nil
}

func (s *Service) seedGroupReconciler(ctx context.Context) error {
	// Seed the user groups with the groups from the backend.
	groups := map[string]types.UserGroup{}
	var nextToken string
	for {
		var userGroups []types.UserGroup
		var err error
		userGroups, nextToken, err = s.accessPoint.ListUserGroups(ctx, 0, nextToken)
		if err != nil {
			return trace.Wrap(err)
		}

		for _, userGroup := range userGroups {
			labels := userGroup.GetStaticLabels()

			// Only look for Okta sourced user groups for this org URL.
			if userGroup.Origin() == types.OriginOkta && labels[teleport.OktaOrgURLLabel] == s.orgURL {
				groups[userGroup.GetName()] = userGroup
			}
		}

		if nextToken == "" {
			break
		}
	}

	s.groupsMu.Lock()
	s.groups = groups
	s.groupsMu.Unlock()

	return nil
}

func (s *Service) startSynchronizerReconcilers(ctx context.Context) error {
	var err error

	if err := s.seedGroupReconciler(ctx); err != nil {
		return trace.Wrap(err)
	}

	s.groupsReconciler, err = services.NewReconciler(services.ReconcilerConfig[types.UserGroup]{
		Matcher:             s.groupMatcher,
		GetCurrentResources: s.getGroups,
		GetNewResources:     s.getNewGroups,
		OnCreate:            s.onCreateGroup,
		OnUpdate:            s.onUpdateGroup,
		OnDelete:            s.onDeleteGroup,
		Log:                 s.log,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	s.appsReconciler, err = services.NewReconciler(services.ReconcilerConfig[*types.AppV3]{
		Matcher:             s.appsMatcher,
		GetCurrentResources: s.getApps,
		GetNewResources:     s.getNewApps,
		OnCreate:            s.onCreateApp,
		OnUpdate:            s.onUpdateApp,
		OnDelete:            s.onDeleteApp,
		Log:                 s.log,
	})

	return trace.Wrap(err)
}

// groupMatcher will match groups.
func (s *Service) groupMatcher(resource types.UserGroup) bool {
	return resource.GetKind() == types.KindUserGroup && resource.Origin() == types.OriginOkta
}

// getGroups returns a copy of the current mapping of user groups.
func (s *Service) getGroups() map[string]types.UserGroup {
	groups := map[string]types.UserGroup{}
	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()

	for k, v := range s.groups {
		groups[k] = v
	}

	return groups
}

// getNewGroups returns a copy of the current mapping of new user groups, unprocessed
// by the reconciler.
func (s *Service) getNewGroups() map[string]types.UserGroup {
	newGroups := map[string]types.UserGroup{}
	s.newGroupsMu.RLock()
	defer s.newGroupsMu.RUnlock()

	for k, v := range s.newGroups {
		newGroups[k] = v
	}

	return newGroups
}

// onCreateGroup will run when a group is created.
func (s *Service) onCreateGroup(ctx context.Context, group types.UserGroup) error {
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return trace.Wrap(err)
	}

	if err := s.accessPoint.CreateUserGroup(ctx, group); err != nil {
		// If the user group already exists, we'll try to update it in the backend instead.
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}

		if err := s.accessPoint.UpdateUserGroup(ctx, group); err != nil {
			return trace.Wrap(err)
		}
	}

	s.groupsMu.Lock()
	s.groups[group.GetName()] = group
	s.groupsMu.Unlock()

	s.addGroupOktaResource(&s.groupsAdded, group)

	return nil
}

// onUpdateGroup will run when a group is updated.
func (s *Service) onUpdateGroup(ctx context.Context, group types.UserGroup) error {
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return trace.Wrap(err)
	}

	if err := s.accessPoint.UpdateUserGroup(ctx, group); err != nil {
		// If the user group already does not exist, we'll try to create it in the backend.
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}

		if err := s.accessPoint.CreateUserGroup(ctx, group); err != nil {
			return trace.Wrap(err)
		}
	}

	s.groupsMu.Lock()
	s.groups[group.GetName()] = group
	s.groupsMu.Unlock()

	s.addGroupOktaResource(&s.groupsUpdated, group)

	return nil
}

// onDeleteGroup will run when a group is deleted.
func (s *Service) onDeleteGroup(ctx context.Context, group types.UserGroup) error {
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return trace.Wrap(err)
	}

	// It's okay to delete a user group that isn't found.
	if err := s.accessPoint.DeleteUserGroup(ctx, group.GetName()); err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	s.groupsMu.Lock()
	delete(s.groups, group.GetName())
	s.groupsMu.Unlock()

	s.addGroupOktaResource(&s.groupsDeleted, group)

	return nil
}

// appMatcher will match applications.
func (s *Service) appsMatcher(resource *types.AppV3) bool {
	return resource.GetKind() == types.KindApp && resource.Origin() == types.OriginOkta
}

// getApps returns a copy of the current mapping of apps.
func (s *Service) getApps() map[string]*types.AppV3 {
	apps := map[string]*types.AppV3{}
	s.appsMu.RLock()
	defer s.appsMu.RUnlock()

	for k, v := range s.apps {
		apps[k] = v
	}

	return apps
}

// getNewApps returns a copy of the current mapping of new apps, unprocessed
// by the reconciler.
func (s *Service) getNewApps() map[string]*types.AppV3 {
	newApps := map[string]*types.AppV3{}
	s.newAppsMu.RLock()
	defer s.newAppsMu.RUnlock()

	for k, v := range s.newApps {
		newApps[k] = v
	}

	return newApps
}

// onCreateApp will run when an application is created.
func (s *Service) onCreateApp(ctx context.Context, app *types.AppV3) error {
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return trace.Wrap(err)
	}

	s.appsMu.Lock()
	s.apps[app.GetName()] = app
	s.appsMu.Unlock()

	if err := s.startHeartbeat(context.Background(), app); err != nil {
		return trace.Wrap(err, "error starting heartbeat for new app %v", app)
	}

	s.addAppOktaResource(&s.appsAdded, app)

	return nil
}

// onUpdateGroup will run when an application is updated.
func (s *Service) onUpdateApp(ctx context.Context, app *types.AppV3) error {
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return trace.Wrap(err)
	}

	s.appsMu.Lock()
	s.apps[app.GetName()] = app
	s.appsMu.Unlock()

	s.addAppOktaResource(&s.appsUpdated, app)

	return nil
}

// onDeleteApp will run when an application is deleted.
func (s *Service) onDeleteApp(ctx context.Context, app *types.AppV3) error {
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return trace.Wrap(err)
	}

	if err := s.stopHeartbeat(app.GetName()); err != nil {
		return trace.Wrap(err, "error stopping heartbeat for deleted app %v", app)
	}

	err := s.accessPoint.DeleteApplicationServer(ctx, defaults.Namespace, s.hostID, app.GetName())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err, "error deleting application server")
	}

	s.appsMu.Lock()
	delete(s.apps, app.GetName())
	s.appsMu.Unlock()

	s.addAppOktaResource(&s.appsDeleted, app)

	return nil
}

// addGroupOktaResources adds the group to the list of Okta resources.
func (s *Service) addGroupOktaResource(target *[]*apievents.OktaResource, group types.UserGroup) {
	*target = append(*target, &apievents.OktaResource{
		ID:          group.GetName(),
		Description: group.GetMetadata().Description,
	})
}

// addAppOktaResource adds the app to the list of Okta resources.
func (s *Service) addAppOktaResource(target *[]*apievents.OktaResource, app types.Application) {
	oktaID, ok := app.GetLabel(teleport.OktaAppIDLabel)
	if !ok {
		s.log.Warnf("app ID label is missing for app %s, using the app name instead", app.GetName())
		oktaID = app.GetName()
	}

	*target = append(*target, &apievents.OktaResource{
		ID:          oktaID,
		Description: app.GetMetadata().Description,
	})
}

// emitSyncEventsInBatches will emit synchronize events in batches so that they're not too large.
func (s *Service) emitSyncEventsInBatches(ctx context.Context, eventName, eventCode string,
	added []*apievents.OktaResource, updated []*apievents.OktaResource, deleted []*apievents.OktaResource) {
	numResourcesAdded := len(added)
	numResourcesUpdated := len(updated)
	numResourcesDeleted := len(deleted)
	total := numResourcesAdded + numResourcesUpdated + numResourcesDeleted

	type batch struct {
		added   []*apievents.OktaResource
		updated []*apievents.OktaResource
		deleted []*apievents.OktaResource
	}

	var batches []*batch
	var currentBatch *batch

	updatedOffset := numResourcesAdded
	deletedOffset := numResourcesAdded + numResourcesUpdated

	for i := 0; i < total; i++ {
		if i%syncEventBatches == 0 {
			currentBatch = &batch{}
			batches = append(batches, currentBatch)
		}

		if i < updatedOffset {
			currentBatch.added = append(currentBatch.added, added[i])
		} else if i >= updatedOffset && i < deletedOffset {
			currentBatch.updated = append(currentBatch.updated, updated[i-updatedOffset])
		} else {
			currentBatch.deleted = append(currentBatch.deleted, deleted[i-deletedOffset])
		}
	}

	for _, batch := range batches {
		event := &apievents.OktaResourcesUpdate{
			Metadata: apievents.Metadata{
				Type: eventName,
				Code: eventCode,
			},
			ServerMetadata: apievents.ServerMetadata{
				ServerID: s.hostID,
			},
			OktaResourcesUpdatedMetadata: apievents.OktaResourcesUpdatedMetadata{
				Added:            int32(len(batch.added)),
				Updated:          int32(len(batch.updated)),
				Deleted:          int32(len(batch.deleted)),
				AddedResources:   batch.added,
				UpdatedResources: batch.updated,
				DeletedResources: batch.deleted,
			},
		}

		if emitErr := s.emitter.EmitAuditEvent(ctx, event); emitErr != nil {
			s.log.WithError(emitErr).Warnf("Failed to emit %s event: %v", eventName, event)
		}
	}
}

func (s *Service) syncUsers(ctx context.Context) error {
	if s.userReconciler == nil {
		s.log.Debug("User synchronization is disabled. Skipping.")
		return nil
	}

	convertUser := makeUserConverter(s.clock, s.ssoConnectorID, s.orgURL)

	oktaUsers, err := fetchOktaUsers(ctx, s.client, convertUser, s.log)
	if err != nil {
		return trace.Wrap(err, "enumerating Okta users")
	}

	teleportUsers, err := listTeleportUsers(ctx, s.accessPoint, s.orgURL, s.log)
	if err != nil {
		return trace.Wrap(err, "listing okta-originated Teleport users")
	}

	s.log.Infof("Reconciling %d Okta and %d Teleport Accounts",
		len(oktaUsers), len(teleportUsers))

	err = s.userReconciler.reconcileUsers(ctx, oktaUsers, teleportUsers)
	if err != nil {
		return trace.Wrap(err, "reconciling teleport users")
	}

	return nil
}
