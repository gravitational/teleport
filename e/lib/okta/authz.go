package okta

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/teleport"
)

const (
	authzTimeout = 5 * time.Second
)

// authorizeTarget will return true if the target should be managed by this Okta service.
func (a *assignmentProcessor) authorizeTarget(ctx context.Context, target types.OktaAssignmentTarget) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, authzTimeout)
	defer cancel()

	switch target.GetTargetType() {
	case constants.OktaAssignmentTargetGroup:
		return a.authorizeUserGroup(ctx, target.GetID())
	case constants.OktaAssignmentTargetApplication:
		return a.authorizeApp(ctx, target.GetID())
	}

	return false, trace.BadParameter("unknown target type %s", target.GetTargetType())
}

// authorizeUserGroup will return true if the user group should be managed by this Okta service.
// This is determined by examining the OktaOrgURLLabel attached to this resource and determining if
// the org URL matches the org URL attached to the Okta service.
func (a *assignmentProcessor) authorizeUserGroup(ctx context.Context, name string) (bool, error) {
	userGroup, err := a.accessPoint.GetUserGroup(ctx, name)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// If the org URLs match, then this prcessor can handle this assignment.
	if orgURL, ok := userGroup.GetLabel(teleport.OktaOrgURLLabel); ok {
		return orgURL == a.oktaOrgURL, nil
	}

	return true, nil
}

// authorizeApp will return true if the app server should be managed by this Okta service.
// This is determined by examining the OktaOrgURLLabel attached to this resource and determining if
// the org URL matches the org URL attached to the Okta service.
func (a *assignmentProcessor) authorizeApp(ctx context.Context, name string) (bool, error) {
	appServer, err := a.getAppServer(ctx, name)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// If the org URLs match, then this prcessor can handle this assignment.
	if orgURL, ok := appServer.GetLabel(teleport.OktaOrgURLLabel); ok {
		return orgURL == a.oktaOrgURL, nil
	}

	return false, nil
}

// getOktaAppIDFromAppServer returns the Okta app ID from the app server.
func (a *assignmentProcessor) getOktaAppIDFromAppServer(ctx context.Context, name string) (string, error) {
	appServer, err := a.getAppServer(ctx, name)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if oktaAppID, ok := appServer.GetLabel(teleport.OktaAppIDLabel); ok && oktaAppID != "" {
		return oktaAppID, nil
	}

	return "", trace.BadParameter(`app_server %q does not have an Okta App ID`, name)
}

// getAppServer will return the singular app server given a name.
func (a *assignmentProcessor) getAppServer(ctx context.Context, name string) (types.AppServer, error) {
	req := proto.ListResourcesRequest{
		ResourceType:        types.KindAppServer,
		Limit:               1,
		PredicateExpression: fmt.Sprintf(`name == %q`, name),
	}

	resp, err := a.accessPoint.ListResources(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(resp.Resources) != 1 {
		return nil, trace.NotFound(`app_server %q doesn't exist`, name)
	}

	appServers, err := types.ResourcesWithLabels(resp.Resources).AsAppServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return appServers[0], nil
}
