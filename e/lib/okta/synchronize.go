package okta

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"

	ossteleport "github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/okta/common"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
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
)

var (
	// SyncRetryAfterLeadershipFailure we will wait for this amount of time before synchronization starts if
	// this Okta service is not the leader.
	SyncRetryAfterLeadershipFailure = time.Minute
)

// synchronizeLoop will synchronize Okta with the backend periodically until the
// process is terminated.
func (s *Service) synchronizeLoop(ctx context.Context) {
	if shouldStop := s.waitIfNotLeader(ctx); shouldStop {
		return
	}

	defer func() {
		s.logger.InfoContext(ctx, "Synchronizer stopped")
		s.syncStoppedChCloser.Do(func() { close(s.syncStoppedCh) })
	}()

	interval := s.getSynchronizerInterval(ctx)

	// Generate a random jitter between 0 and 10 seconds
	timer := s.clock.NewTimer(interval + utils.RandomDuration(syncJitter))
	defer timer.Stop()

	s.logger.InfoContext(ctx, "Synchronizer started", "refresh_interval", interval)

	for {
		s.synchronizeAndEmitEvents(ctx)

		select {
		case <-timer.Chan():
			interval = s.getSynchronizerInterval(ctx)
			timer.Reset(interval + utils.RandomDuration(syncJitter))
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// waitIfNotLeader will wait for this service to become the leader. It will return true if the service should stop.
func (s *Service) waitIfNotLeader(ctx context.Context) bool {
	// Don't start the loop until we acquire leadership.
	s.logger.InfoContext(ctx, "Waiting for leadership to be acquired before starting synchronizer")
	waitForLeadershipTicker := s.clock.NewTicker(SyncRetryAfterLeadershipFailure)
	defer waitForLeadershipTicker.Stop()
	for {
		if s.leader.IsLeader() {
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
			ServerVersion: ossteleport.Version,
			ServerID:      s.hostID,
		},
		Status: apievents.Status{
			Success: false,
			Error:   err.Error(),
		},
	}

	if emitErr := s.emitter.EmitAuditEvent(ctx, event); emitErr != nil {
		s.logger.WarnContext(ctx, "Failed to emit Okta synchronization failure event", "event_type", event.GetType(), "error", emitErr)
	}
}

// getSynchronizerInterval creates the ticker for the synchronizer.
func (s *Service) getSynchronizerInterval(ctx context.Context) time.Duration {
	timeBetweenSyncs := s.timeBetweenSyncs
	pref, err := s.accessPoint.GetAuthPreference(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "Error getting configure okta sync interval, using default", "default_interval", timeBetweenSyncs, "error", err)
	} else {
		if pref.GetOktaSyncPeriod() != 0 {
			timeBetweenSyncs = pref.GetOktaSyncPeriod()
		}
	}

	return timeBetweenSyncs
}

// synchronizeAndEmitEvents will run synchronization, emit events on success or failure, and mark the synchronization
// successful if no errors were encountered.
func (s *Service) synchronizeAndEmitEvents(ctx context.Context) {
	// If the parent Okta service is not the leader, skip synchronizing.
	if !s.leader.IsLeader() {
		return
	}

	// Add to the synchronizing wait group so that the importer waits if we're actively synchronizing.
	s.synchronizingMu.Lock()
	defer s.synchronizingMu.Unlock()

	err := s.synchronize(ctx)

	s.serviceStatus.UpdateAppGroupSync(ctx, s.clock.Now(), s.apps.Len(), s.groups.Len(), err)

	if err != nil {
		s.logger.ErrorContext(ctx, "Error while synchronizing Okta resources with Teleport", "error", err)
		s.emitSyncError(ctx, err)
	} else {
		// The synchronizer has completed at least once successfully. This will allow the access
		// list sync to proceed.
		s.synchronizerSuccess.Store(true)
	}
}

// synchronize will synchronize the Okta groups and applications with the backend.
func (s *Service) synchronize(ctx context.Context) error {
	if err := s.syncUsers(ctx); err != nil {
		return trace.Wrap(err)
	}

	if !s.disableOktaAppGroupSync {
		if err := s.buildImportRuleMappings(ctx); err != nil {
			return trace.Wrap(err)
		}
		groupsToAppsMapping, err := s.synchronizeApplications(ctx)
		if err != nil {
			s.logger.WarnContext(ctx, "Error when synchronizing applications, unable to sync groups", "error", err)
			// We need the groups to apps mapping in order to synchronize groups properly, so
			// we won't try to synchronize groups if we can't synchronize apps.
			return trace.Wrap(err)
		}
		if err := s.synchronizeGroups(ctx, groupsToAppsMapping); err != nil {
			s.logger.WarnContext(ctx, "Error when synchronizing groups", "error", err)
			return trace.Wrap(err)
		}
	}

	return nil
}

// synchronizeGroups will synchronize Okta groups with the backend.
func (s *Service) synchronizeGroups(ctx context.Context, groupsToAppsMapping userGroupsToApplications) error {
	if s.groupsReconciler == nil {
		s.logger.DebugContext(ctx, "Group synchronization is disabled")
		return nil
	}
	newGroups := map[string]types.UserGroup{}
	err := s.client.IterateGroups(ctx, func(oktaGroup *okta.Group) error {
		s.logger.DebugContext(ctx, "Processing Okta group", "group_id", oktaGroup.Id)

		userGroup, err := s.oktaGroupToUserGroup(oktaGroup, groupsToAppsMapping[oktaGroup.Id])
		if err != nil {
			s.logger.DebugContext(ctx, "Error converting Okta group", "error", err)
			return nil
		}

		newGroups[userGroup.GetName()] = userGroup

		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	s.newGroups.Set(newGroups)

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
	if s.appsReconciler == nil {
		s.logger.DebugContext(ctx, "Application synchronization is disabled")
		return nil, nil
	}
	s.logger.DebugContext(ctx, "Synchronizing applications")

	groupsToAppsMapping := userGroupsToApplications{}
	newApps := map[string]types.Application{}
	err := s.client.IterateApps(ctx, func(oktaApp okta.App) error {
		// This type assertion is necessary as okta.App, which is supplied by the Okta go SDK,
		// does not contain all of the information that we need to create a types.Application
		// object.
		oktaApplication, ok := oktaApp.(*okta.Application)
		if !ok {
			s.logger.DebugContext(ctx, "Unable to process Okta application of unknown type")
			return nil
		}

		logger := s.logger.With("application_id", oktaApplication.Id)
		logger.DebugContext(ctx, "Processing Okta application")

		oktaGroups, err := s.client.GetAppGroups(ctx, oktaAppID(oktaApplication.Id))
		if err != nil {
			s.logger.WarnContext(ctx, "Error getting groups for application", "error", err)
			if !trace.IsNotFound(err) {
				return trace.Wrap(err, "getting groups for application %q", oktaApplication.Id)
			}
			return nil
		}

		groups := make([]string, len(oktaGroups))
		for i, g := range oktaGroups {
			groups[i] = string(g)
		}

		apps, err := s.oktaAppToApp(oktaApplication, groups)
		if err != nil {
			s.logger.DebugContext(ctx, "Error converting Okta app", "error", err)
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

	s.newApps.Set(newApps)

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
			if userGroup.Origin() == types.OriginOkta && labels[eteleport.OktaOrgURLLabel] == s.orgURL {
				groups[userGroup.GetName()] = userGroup
			}
		}

		if nextToken == "" {
			break
		}
	}
	s.groups.Set(groups)
	return nil
}

func (s *Service) startSynchronizerReconcilers(ctx context.Context) error {
	if s.disableOktaAppGroupSync {
		return nil
	}

	var err error
	if err := s.seedGroupReconciler(ctx); err != nil {
		return trace.Wrap(err)
	}

	s.groupsReconciler, err = services.NewReconciler(services.ReconcilerConfig[types.UserGroup]{
		Matcher:             s.groupMatcher,
		GetCurrentResources: s.groups.Clone,
		GetNewResources:     s.newGroups.Clone,
		OnCreate:            s.onCreateGroup,
		OnUpdate:            s.onUpdateGroup,
		OnDelete:            s.onDeleteGroup,
		Logger:              s.logger.With("kind", types.KindUserGroup),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	s.appsReconciler, err = services.NewReconciler(services.ReconcilerConfig[types.Application]{
		Matcher:             s.appsMatcher,
		GetCurrentResources: s.apps.Clone,
		GetNewResources:     s.newApps.Clone,
		OnCreate:            s.onCreateApp,
		OnUpdate:            s.onUpdateApp,
		OnDelete:            s.onDeleteApp,
		Logger:              s.logger.With("kind", types.KindAppServer),
	})

	return trace.Wrap(err)
}

// groupMatcher will match groups.
func (s *Service) groupMatcher(resource types.UserGroup) bool {
	return resource.GetKind() == types.KindUserGroup && resource.Origin() == types.OriginOkta
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

	s.groups.Store(group.GetName(), group)

	s.addGroupOktaResource(&s.groupsAdded, group)

	return nil
}

// onUpdateGroup will run when a group is updated.
func (s *Service) onUpdateGroup(ctx context.Context, group, _ types.UserGroup) error {
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

	s.groups.Store(group.GetName(), group)

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

	s.groups.Delete(group.GetName())

	s.addGroupOktaResource(&s.groupsDeleted, group)

	return nil
}

// appMatcher will match applications.
func (s *Service) appsMatcher(resource types.Application) bool {
	return resource.GetKind() == types.KindApp && resource.Origin() == types.OriginOkta
}

// onCreateApp will run when an application is created.
func (s *Service) onCreateApp(ctx context.Context, app types.Application) error {
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return trace.Wrap(err)
	}

	s.apps.Store(app.GetName(), app)

	if err := s.startHeartbeat(context.Background(), app); err != nil {
		return trace.Wrap(err, "error starting heartbeat for new app %v", app)
	}

	s.addAppOktaResource(ctx, &s.appsAdded, app)

	return nil
}

// onUpdateGroup will run when an application is updated.
func (s *Service) onUpdateApp(ctx context.Context, app, _ types.Application) error {
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return trace.Wrap(err)
	}

	s.apps.Store(app.GetName(), app)

	s.addAppOktaResource(ctx, &s.appsUpdated, app)

	return nil
}

// onDeleteApp will run when an application is deleted.
func (s *Service) onDeleteApp(ctx context.Context, app types.Application) error {
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

	s.apps.Delete(app.GetName())

	s.addAppOktaResource(ctx, &s.appsDeleted, app)

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
func (s *Service) addAppOktaResource(ctx context.Context, target *[]*apievents.OktaResource, app types.Application) {
	oktaID, ok := app.GetLabel(eteleport.OktaAppIDLabel)
	if !ok {
		s.logger.WarnContext(ctx, "app ID label is missing for app, using the app name instead", "app", app.GetName())
		oktaID = app.GetName()
	}

	*target = append(*target, &apievents.OktaResource{
		ID:          oktaID,
		Description: app.GetMetadata().Description,
	})
}

// emitSyncEventsInBatches will emit synchronize events in batches so that they're not too large.
func (s *Service) emitSyncEventsInBatches(ctx context.Context, eventName, eventCode string,
	added []*apievents.OktaResource, updated []*apievents.OktaResource, deleted []*apievents.OktaResource,
) {
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
				ServerVersion: ossteleport.Version,
				ServerID:      s.hostID,
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
			s.logger.WarnContext(ctx, "Failed to emit %s event: %v",
				"event_name", eventName,
				"event_type", event.GetType(),
				"error", emitErr,
			)
		}
	}
}

func (s *Service) syncUsers(ctx context.Context) (err error) {
	defer func() {
		// Always update the failure state if we exit with a non-nil error
		if err != nil {
			s.logger.ErrorContext(ctx, "Setting error flag")
			s.serviceStatus.UpdateUserSync(ctx, s.clock.Now(), 0, err)
		}
	}()

	if s.userReconciler == nil {
		s.logger.DebugContext(ctx, "User synchronization is disabled")
		return nil
	}

	var oktaUsers map[string]types.User
	if s.oktaSAMLAppID != "" {
		s.logger.DebugContext(ctx, "Fetching app users", "app_id", s.oktaSAMLAppID)
		convertUser := func(oktaUser *okta.AppUser) (types.User, error) {
			return ConvertAppUser(oktaUser, s.clock, s.ssoConnectorID, s.orgURL)
		}
		oktaUsers, err = fetchOktaAppUsers(ctx, s.client, s.oktaSAMLAppID, convertUser, s.logger)
	} else {
		s.logger.DebugContext(ctx, "Fetching org users")
		convertUser := makeUserConverter(s.clock, s.ssoConnectorID, s.orgURL)
		oktaUsers, err = fetchOktaUsers(ctx, s.client, convertUser, s.logger)
	}
	if err != nil {
		return trace.Wrap(err, "enumerating Okta users")
	}

	teleportUsers, err := listTeleportUsers(ctx, s.accessPoint, s.orgURL)
	if err != nil {
		return trace.Wrap(err, "listing okta-originated Teleport users")
	}

	connector, err := s.connectorService.GetSAMLConnector(ctx, s.ssoConnectorID, false)
	if err != nil {
		return trace.Wrap(err, "getting SAML connector")
	}
	for _, user := range oktaUsers {
		if err := s.calcUserTraits(ctx, connector, user); err != nil {
			return trace.Wrap(err, "calculating traits")
		}
	}

	s.logger.InfoContext(ctx, "Reconciling Okta and Teleport Accounts",
		"okta_user_count", len(oktaUsers),
		"teleport_user_count", len(teleportUsers),
	)
	stats, err := s.userReconciler.reconcileUsers(ctx, oktaUsers, teleportUsers)
	if err != nil {
		return trace.Wrap(err, "reconciling teleport users")
	}
	s.serviceStatus.UpdateUserSync(ctx, s.clock.Now(), stats.total(), nil)
	return nil
}

func (s *Service) calcUserTraits(ctx context.Context, connector types.SAMLConnector, user types.User) error {
	oktaUserID, ok := user.GetLabel(eteleport.OktaUserIDLabel)
	if !ok {
		return trace.BadParameter("user missing Okta user ID")
	}
	groups, err := s.client.ListUserGroups(ctx, oktaUserID)
	if err != nil {
		return trace.Wrap(err, "listing user groups")
	}
	var groupsList []string
	for _, v := range groups {
		groupsList = append(groupsList, v.Name)
	}
	common.SetUserRolesAndTraits(user, groupsList, connector)
	return nil
}
