/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package okta

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// synchronizeLoop will synchronize Okta with the backend periodically until the
// process is terminated.
func (s *Service) synchronizeLoop(ctx context.Context) {
	// Generate a random jitter between 0 and 10 seconds
	ticker := s.clock.NewTicker(s.timeBetweenSyncs + utils.RandomDuration(10000*time.Millisecond))
	defer ticker.Stop()

Loop:
	for {
		timeoutCtx, cancel := context.WithTimeout(ctx, s.timeBetweenSyncs)
		if err := s.synchronize(timeoutCtx); err != nil {
			s.log.Errorf("Error while synchronizing Okta resources with Teleport: %v", err)
		}
		cancel()

		select {
		case <-ticker.Chan():
		case <-s.stopCh:
			break Loop
		case <-ctx.Done():
			break Loop
		}
	}

	s.syncStoppedChCloser.Do(func() { close(s.syncStoppedCh) })
}

// synchronize will synchronize the Okta groups and applications with the backend.
func (s *Service) synchronize(ctx context.Context) error {
	if err := s.buildImportRuleMappings(ctx); err != nil {
		return trace.Wrap(err)
	}

	if err := s.synchronizeGroups(ctx); err != nil {
		s.log.Warnf("Error when synchronizing groups: %v", err)
	}

	if err := s.synchronizeApplications(ctx); err != nil {
		s.log.Warnf("Error when synchronizing applications: %v", err)
	}

	return nil
}

// synchronizeGroups will synchronize Okta groups with the backend.
func (s *Service) synchronizeGroups(ctx context.Context) error {
	newGroups := map[string]types.UserGroup{}
	err := s.client.iterateGroups(ctx, func(oktaGroup *okta.Group) error {
		s.log.Debugf("Processing Okta group %v", oktaGroup.Id)

		userGroup, err := s.oktaGroupToUserGroup(oktaGroup)
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

	if err := s.groupsReconciler.Reconcile(ctx); err != nil {
		return trace.Wrap(err, "error during group reconciliation")
	}

	return nil
}

// synchronizeApplications will synchronize Okta applications with the backend.
func (s *Service) synchronizeApplications(ctx context.Context) error {
	s.log.Debug("Synchronizing applications")

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

		apps, err := s.oktaAppToApp(oktaApplication)
		if err != nil {
			s.log.Debugf("Error converting Okta app: %v", err)
			return nil
		}

		for _, app := range apps {
			newApps[app.GetName()] = app
		}

		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	s.newAppsMu.Lock()
	s.newApps = newApps
	s.newAppsMu.Unlock()

	if err := s.appsReconciler.Reconcile(ctx); err != nil {
		return trace.Wrap(err, "error during application reconciliation")
	}

	return nil
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
			if userGroup.Origin() == types.OriginOkta && labels[oktaOrgURLLabel] == s.orgURL {
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

func (s *Service) startReconcilers(ctx context.Context) error {
	var err error

	if err := s.seedGroupReconciler(ctx); err != nil {
		return trace.Wrap(err)
	}

	s.groupsReconciler, err = services.NewReconciler(services.ReconcilerConfig{
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

	s.appsReconciler, err = services.NewReconciler(services.ReconcilerConfig{
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
func (s *Service) groupMatcher(resource types.ResourceWithLabels) bool {
	return resource.GetKind() == types.KindUserGroup && resource.Origin() == types.OriginOkta
}

// getGroups returns a copy of the current mapping of user groups.
func (s *Service) getGroups() types.ResourcesWithLabelsMap {
	groups := types.ResourcesWithLabelsMap{}
	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()

	for k, v := range s.groups {
		groups[k] = v
	}

	return groups
}

// getNewGroups returns a copy of the current mapping of new user groups, unprocessed
// by the reconciler.
func (s *Service) getNewGroups() types.ResourcesWithLabelsMap {
	newGroups := types.ResourcesWithLabelsMap{}
	s.newGroupsMu.RLock()
	defer s.newGroupsMu.RUnlock()

	for k, v := range s.newGroups {
		newGroups[k] = v
	}

	return newGroups
}

// onCreateGroup will run when a group is created.
func (s *Service) onCreateGroup(ctx context.Context, resource types.ResourceWithLabels) error {
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return trace.Wrap(err)
	}

	group, ok := resource.(types.UserGroup)
	if !ok {
		return trace.BadParameter("expected type types.UserGroup, got %T", resource)
	}

	if err := s.accessPoint.CreateUserGroup(ctx, group); err != nil {
		return trace.Wrap(err)
	}

	s.groupsMu.Lock()
	s.groups[group.GetName()] = group
	s.groupsMu.Unlock()

	return nil
}

// onUpdateGroup will run when a group is updated.
func (s *Service) onUpdateGroup(ctx context.Context, resource types.ResourceWithLabels) error {
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return trace.Wrap(err)
	}

	group, ok := resource.(types.UserGroup)
	if !ok {
		return trace.BadParameter("expected type types.UserGroup, got %T", resource)
	}

	if err := s.accessPoint.UpdateUserGroup(ctx, group); err != nil {
		return trace.Wrap(err)
	}

	s.groupsMu.Lock()
	s.groups[group.GetName()] = group
	s.groupsMu.Unlock()

	return nil
}

// onDeleteGroup will run when a group is deleted.
func (s *Service) onDeleteGroup(ctx context.Context, resource types.ResourceWithLabels) error {
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return trace.Wrap(err)
	}

	if err := s.accessPoint.DeleteUserGroup(ctx, resource.GetName()); err != nil {
		return trace.Wrap(err)
	}

	s.groupsMu.Lock()
	delete(s.groups, resource.GetName())
	s.groupsMu.Unlock()

	return nil
}

// appMatcher will match applications.
func (s *Service) appsMatcher(resource types.ResourceWithLabels) bool {
	return resource.GetKind() == types.KindApp && resource.Origin() == types.OriginOkta
}

// getApps returns a copy of the current mapping of apps.
func (s *Service) getApps() types.ResourcesWithLabelsMap {
	apps := types.ResourcesWithLabelsMap{}
	s.appsMu.RLock()
	defer s.appsMu.RUnlock()

	for k, v := range s.apps {
		apps[k] = v
	}

	return apps
}

// getNewApps returns a copy of the current mapping of new apps, unprocessed
// by the reconciler.
func (s *Service) getNewApps() types.ResourcesWithLabelsMap {
	newApps := types.ResourcesWithLabelsMap{}
	s.newAppsMu.RLock()
	defer s.newAppsMu.RUnlock()

	for k, v := range s.newApps {
		newApps[k] = v
	}

	return newApps
}

// onCreateApp will run when an application is created.
func (s *Service) onCreateApp(ctx context.Context, resource types.ResourceWithLabels) error {
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return trace.Wrap(err)
	}

	app, ok := resource.(*types.AppV3)
	if !ok {
		return trace.BadParameter("expected type types.Application, got %T", resource)
	}

	s.appsMu.Lock()
	s.apps[app.GetName()] = app
	s.appsMu.Unlock()

	if err := s.startHeartbeat(context.Background(), app); err != nil {
		return trace.Wrap(err, "error starting heartbeat for new app %v", app)
	}

	return nil
}

// onUpdateGroup will run when an application is updated.
func (s *Service) onUpdateApp(ctx context.Context, resource types.ResourceWithLabels) error {
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return trace.Wrap(err)
	}

	app, ok := resource.(*types.AppV3)
	if !ok {
		return trace.BadParameter("expected type types.Application, got %T", resource)
	}

	s.appsMu.Lock()
	s.apps[app.GetName()] = app
	s.appsMu.Unlock()

	return nil
}

// onDeleteApp will run when an application is deleted.
func (s *Service) onDeleteApp(ctx context.Context, resource types.ResourceWithLabels) error {
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return trace.Wrap(err)
	}

	app, ok := resource.(*types.AppV3)
	if !ok {
		return trace.BadParameter("expected type types.Application, got %T", resource)
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

	return nil
}
