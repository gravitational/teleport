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
)

// authorizeTarget will return true if the target should be managed by this Okta service.
func (a *assignmentProcessor) authorizeTarget(target types.OktaAssignmentTarget) authResult {
	var resource types.ResourceWithLabels
	switch target.GetTargetType() {
	case constants.OktaAssignmentTargetGroup:
		userGroup, ok := a.syncedUserGroups.Load(target.GetID())
		if !ok {
			return targetNotFound
		}
		resource = userGroup
	case constants.OktaAssignmentTargetApplication:
		appServer, ok := a.syncedAppServers.Load(target.GetID())
		if !ok {
			return targetNotFound
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
