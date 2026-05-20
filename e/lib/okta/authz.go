package okta

import (
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/teleport"
)

type authResult string

const (
	targetAuthorized      = "authorized"
	targetNotFound        = "not_found"
	targetOktaOrgMismatch = "okta_org_mismatch"
	targetNotIncluded     = "not_included"
)

// authorizeTarget will return true if the target should be managed by this Okta service.
// The target is authorized if the following conditions are met:
//   - The target exists and is an app or group.
//   - The target name label matches a filter.
//   - The target okta/org label matches the label on the assignment.
func (a *assignmentProcessor) authorizeTarget(target types.OktaAssignmentTarget) authResult {
	var resource types.ResourceWithLabels
	switch target.GetTargetType() {
	case constants.OktaAssignmentTargetGroup:
		userGroup, ok := a.syncedUserGroups.Load(target.GetID())
		if !ok {
			return targetNotFound
		}
		userGroupName, _ := userGroup.GetLabel(types.OktaGroupNameLabel)
		// If the user group isn't matched by filters then it is not synced back to Okta.
		// This prevents narrowing of group filters from unexpectedly unassigning users in Okta.
		if !matchesAnyFilter(a.groupFilters, userGroupName) {
			return targetNotIncluded
		}
		resource = userGroup
	case constants.OktaAssignmentTargetApplication:
		appServer, ok := a.syncedAppServers.Load(target.GetID())
		if !ok {
			return targetNotFound
		}
		appServerName, _ := appServer.GetLabel(types.OktaAppNameLabel)
		// If the app server isn't matched by filters then it is not synced back to Okta.
		// This prevents narrowing of app filters from unexpectedly unassigning users in Okta.
		if !matchesAnyFilter(a.appFilters, appServerName) {
			return targetNotIncluded
		}
		resource = appServer
	}

	if orgURL, _ := resource.GetLabel(teleport.OktaOrgURLLabel); orgURL != a.oktaOrgURL {
		return targetOktaOrgMismatch
	}

	return targetAuthorized
}

func (a *assignmentProcessor) getOktaAppIDFromAppServer(name string) (oktaAppID, bool) {
	app, ok := a.syncedAppServers.Load(name)
	if !ok {
		return "", false
	}
	appID, _ := app.GetLabel(teleport.OktaAppIDLabel)
	if appID == "" {
		return "", false
	}
	return oktaAppID(appID), true
}
