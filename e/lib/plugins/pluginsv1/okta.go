package pluginsv1

import (
	"context"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
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
	if !slices.Equal(oktaRequesterRole.GetSearchAsRoles(types.Allow), services.NewSystemOktaRequesterRole().GetSearchAsRoles(types.Allow)) {
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
		if err := s.authServer.DeleteOktaAssignment(ctx, oktaAssignmentName); err != nil {
			return trace.Wrap(err)
		}
	}

	oktaAccessListNames, err := s.getOktaAccessListNames(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, accessListName := range oktaAccessListNames {
		s.logger.InfoContext(ctx, "Deleting Okta access list", "access_list_name", accessListName)
		if err := s.authServer.DeleteAccessList(ctx, accessListName); err != nil {
			return trace.Wrap(err)
		}
	}

	oktaRoleNames, err := s.getOktaRoleNames(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, roleName := range oktaRoleNames {
		s.logger.InfoContext(ctx, "Deleting Okta role", "role_name", roleName)
		if err := s.authServer.DeleteRole(ctx, roleName); err != nil {
			return trace.Wrap(err)
		}
	}

	// FInally, make sure the Okta requester role is reset.
	_, err = s.authServer.UpsertRole(ctx, services.NewSystemOktaRequesterRole())
	if err != nil {
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
