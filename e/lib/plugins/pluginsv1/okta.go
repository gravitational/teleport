package pluginsv1

import (
	"context"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

// oktaNeedsCleanup will return an error if an Okta plugin needs cleanup.
func (s *Service) oktaNeedsCleanup(ctx context.Context) ([]*types.ResourceID, bool, error) {
	active, err := s.isPluginOfTypeActive(ctx, types.PluginTypeOkta)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}

	var allResources []*types.ResourceID
	oktaAssignmentNames, err := s.getOktaAssignmentNames(ctx)
	if err != nil {
		return nil, active, trace.Wrap(err)
	}
	for _, oktaAssignmentName := range oktaAssignmentNames {
		allResources = append(allResources, &types.ResourceID{Kind: types.KindOktaAssignment, Name: oktaAssignmentName})
	}

	oktaAccessListNames, err := s.getOktaAccessListNames(ctx)
	if err != nil {
		return nil, active, trace.Wrap(err)
	}
	for _, oktaAccessListName := range oktaAccessListNames {
		allResources = append(allResources, &types.ResourceID{Kind: types.KindAccessList, Name: oktaAccessListName})
	}

	oktaAppServers, err := s.getOktaAppServers(ctx)
	if err != nil {
		return nil, active, trace.Wrap(err)
	}
	for _, oktaAppServer := range oktaAppServers {
		allResources = append(allResources, &types.ResourceID{Kind: types.KindAppServer, Name: oktaAppServer.GetHostID() + "/" + oktaAppServer.GetName()})
	}

	oktaUserGroups, err := s.getOktaUserGroups(ctx)
	if err != nil {
		return nil, active, trace.Wrap(err)
	}
	for _, oktaUserGroup := range oktaUserGroups {
		allResources = append(allResources, &types.ResourceID{Kind: types.KindUserGroup, Name: oktaUserGroup.GetName()})
	}

	oktaRoleNames, err := s.getOktaRoleNames(ctx)
	if err != nil {
		return nil, active, trace.Wrap(err)
	}
	for _, oktaRoleName := range oktaRoleNames {
		allResources = append(allResources, &types.ResourceID{Kind: types.KindRole, Name: oktaRoleName})
	}

	// Mark if the Okta requester role needs to be cleaned up as well.
	oktaRequesterRole, err := s.authServer.GetRole(ctx, teleport.SystemOktaRequesterRoleName)
	if err != nil {
		return nil, active, trace.Wrap(err)
	}

	// Check to see if search as roles is the same as the preset.
	if !slices.Equal(oktaRequesterRole.GetSearchAsRoles(types.Allow), services.NewSystemOktaRequesterRole(modules.BuildEnterprise).GetSearchAsRoles(types.Allow)) {
		allResources = append(allResources, &types.ResourceID{Kind: types.KindRole, Name: teleport.SystemOktaRequesterRoleName})
	}

	return allResources, active, nil
}

// cleanupOkta will clean up Okta resources.
func (s *Service) cleanupOkta(ctx context.Context) error {
	active, err := s.isPluginOfTypeActive(ctx, types.PluginTypeOkta)
	if err != nil {
		return trace.Wrap(err)
	}

	if active {
		return trace.CompareFailed("an Okta plugin is configured, can't cleanup")
	}

	s.logger.InfoContext(ctx, "Cleaning up Okta plugin resources")
	oktaAssignmentNames, err := s.getOktaAssignmentNames(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, oktaAssignmentName := range oktaAssignmentNames {
		s.logger.InfoContext(ctx, "Deleting Okta assignment", "assignment_name", oktaAssignmentName)
		if err := s.authServer.DeleteOktaAssignment(ctx, oktaAssignmentName); err != nil && !trace.IsNotFound(err) {
			s.logger.ErrorContext(ctx, "Failed to delete okta_assignment during plugin cleanup", "okta_assignment_name", oktaAssignmentName, "error", err)
			return trace.Wrap(err)
		}
	}

	oktaAccessListNames, err := s.getOktaAccessListNames(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, accessListName := range oktaAccessListNames {
		s.logger.InfoContext(ctx, "Deleting Okta access list", "access_list_name", accessListName)
		if err := s.authServer.DeleteAccessList(ctx, accessListName); err != nil && !trace.IsNotFound(err) {
			s.logger.ErrorContext(ctx, "Failed to delete access_list during plugin cleanup", "access_list_name", accessListName, "error", err)
			return trace.Wrap(err)
		}
	}

	appServers, err := s.getOktaAppServers(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, as := range appServers {
		labels := as.GetStaticLabels()
		oktaID, oktaName := labels[types.OktaAppIDLabel], labels[types.OktaAppNameLabel]
		asName, asHostID := as.GetName(), as.GetHostID()
		s.logger.InfoContext(ctx, "Deleting Okta app server", "app_server_name", asName, "okta_app_id", oktaID, "okta_app_name", oktaName)
		if err := s.authServer.DeleteApplicationServer(ctx, defaults.Namespace, asHostID, asName); err != nil && !trace.IsNotFound(err) {
			s.logger.ErrorContext(ctx, "Failed to delete app_server during plugin cleanup", "app_server_host_id", asHostID, "app_server_name", asName, "error", err)
			return trace.Wrap(err)
		}
	}

	userGroups, err := s.getOktaUserGroups(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, group := range userGroups {
		labels := group.GetStaticLabels()
		oktaName := labels[types.OktaGroupNameLabel]
		groupName := group.GetName()
		s.logger.InfoContext(ctx, "Deleting Okta user group", "user_group_name", groupName, "okta_app_id", groupName, "okta_group_name", oktaName)
		if err := s.authServer.DeleteUserGroup(ctx, groupName); err != nil && !trace.IsNotFound(err) {
			s.logger.ErrorContext(ctx, "Failed to delete user_group during plugin cleanup", "user_group_name", groupName, "error", err)
			return trace.Wrap(err)
		}
	}

	oktaRoleNames, err := s.getOktaRoleNames(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, roleName := range oktaRoleNames {
		s.logger.InfoContext(ctx, "Deleting Okta role", "role_name", roleName)
		if err := s.authServer.DeleteRole(ctx, roleName); err != nil && !trace.IsNotFound(err) {
			s.logger.ErrorContext(ctx, "Failed to delete role during plugin cleanup", "user_group_name", roleName, "error", err)
			return trace.Wrap(err)
		}
	}

	// FInally, make sure the Okta requester role is reset.
	s.logger.InfoContext(ctx, "Resetting okta-requester role")
	if _, err = s.authServer.UpsertRole(ctx, services.NewSystemOktaRequesterRole(modules.BuildEnterprise)); err != nil {
		s.logger.ErrorContext(ctx, "Failed to reset okta-requester role during plugin cleanup", "error", err)
		return trace.Wrap(err)
	}

	return nil
}

// getOktaAssignmentNames returns a list of all Okta assignment names.
func (s *Service) getOktaAssignmentNames(ctx context.Context) ([]string, error) {
	pageToken := ""

	var names []string
	for {
		var assignments []types.OktaAssignment
		var err error
		assignments, pageToken, err = s.authServer.ListOktaAssignments(ctx, 0, pageToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, assignment := range assignments {
			names = append(names, assignment.GetName())
		}

		if pageToken == "" {
			break
		}
	}

	return names, nil
}

// getOktaAccessListNames returns a list of all Okta sourced access list names.
func (s *Service) getOktaAccessListNames(ctx context.Context) ([]string, error) {
	pageToken := ""

	var names []string
	for {
		var accessLists []*accesslist.AccessList
		var err error
		accessLists, pageToken, err = s.authServer.ListAccessLists(ctx, 0, pageToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, accessList := range accessLists {
			if accessList.Origin() == types.OriginOkta {
				names = append(names, accessList.GetName())
			}
		}

		if pageToken == "" {
			break
		}
	}

	return names, nil
}

// getOktaAppServers returns a list of all Okta sourced app servers.
func (s *Service) getOktaAppServers(ctx context.Context) ([]types.AppServer, error) {
	appServers, err := s.authServer.GetApplicationServers(ctx, defaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var res []types.AppServer
	for _, as := range appServers {
		if as.Origin() == types.OriginOkta {
			res = append(res, as)
		}
	}
	return res, nil
}

// getOktaUserGroups returns a list of all Okta sourced user groups.
func (s *Service) getOktaUserGroups(ctx context.Context) ([]types.UserGroup, error) {
	var res []types.UserGroup
	for group, err := range clientutils.Resources(ctx, s.authServer.ListUserGroups) {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if group.Origin() == types.OriginOkta {
			res = append(res, group)
		}
	}
	return res, nil
}

// getOktaRoles returns a list of all Okta sourced roles.
func (s *Service) getOktaRoleNames(ctx context.Context) ([]string, error) {
	pageToken := ""

	var names []string
	for {
		resp, err := s.authServer.ListRoles(ctx, &proto.ListRolesRequest{
			StartKey: pageToken,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		pageToken = resp.NextKey

		for _, role := range resp.Roles {
			if role.Origin() == types.OriginOkta && role.GetName() != teleport.SystemOktaRequesterRoleName {
				names = append(names, role.GetName())
			}
		}

		if pageToken == "" {
			break
		}
	}

	return names, nil
}
