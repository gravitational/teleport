package service

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"

	netiqclient "github.com/gravitational/teleport/e/lib/netiq/client"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

func (s *Service) pullNetIQData(ctx context.Context) (results resources, err error) {
	users, err := s.client.ListUsers(ctx)
	if err != nil {
		return results, trace.Wrap(err, "failed to list users")
	}
	results.users = convertUsers(users)

	resources, err := s.client.ListResources(ctx)
	if err != nil {
		return results, trace.Wrap(err, "failed to list resources")
	}
	results.resources = convertResources(resources)

	groups, err := s.client.ListGroups(ctx)
	if err != nil {
		return results, trace.Wrap(err, "failed to list groups")
	}
	results.groups = convertGroups(groups)

	roles, err := s.client.ListRoles(ctx)
	if err != nil {
		return results, trace.Wrap(err, "failed to list roles")
	}
	results.roles = convertRoles(roles)

	const maxParallelRequests = 15
	var (
		groupMemberships = make(chan groupMembership, len(groups))
		roleMemberships  = make(chan roleMembership, len(roles))
		errs             = make(chan error, len(groups)+len(roles))
	)
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxParallelRequests)

	for _, group := range groups {
		g.Go(func() error {
			groupMembership, err := s.collectGroupMemberships(ctx, group)
			if err != nil {
				errs <- err
				return nil
			}
			groupMemberships <- groupMembership
			return nil
		})
	}

	for _, role := range roles {
		g.Go(func() error {
			roleMembership, err := s.collectRoleMemberships(ctx, role)
			if err != nil {
				errs <- err
				return nil
			}
			roleMemberships <- roleMembership
			return nil
		})
	}

	var collectedErrs []error
	// every goroutine writes to exactly one channel then returns,
	// so iterate for each goroutine and collect from one channel
	for range len(groups) + len(roles) {
		select {
		case err := <-errs:
			collectedErrs = append(collectedErrs, err)
		case groupMember := <-groupMemberships:
			results.groupMembers = append(results.groupMembers, convertGroupMembers(groupMember.groupID, groupMember.members)...)
		case roleMember := <-roleMemberships:
			results.roleMembers = append(results.roleMembers, convertRoleMembers(roleMember.roleID, roleMember.members)...)
			results.mappedResources = append(results.mappedResources, convertRoleMappedResources(roleMember.roleID, roleMember.mappedResources)...)
			results.parentRoles = append(results.parentRoles, convertRoleParentRoles(roleMember.roleID, roleMember.parentRoles)...)
		}
	}

	_ = g.Wait() // ignore the error because we are collecting them in errs

	return results, trace.NewAggregate(collectedErrs...)
}

type groupMembership struct {
	groupID string
	members []netiqclient.GroupMember
}

// collectGroupMemberships collects the group memberships for the given group.
func (s *Service) collectGroupMemberships(ctx context.Context, group netiqclient.Group) (groupMembership, error) {
	members, err := s.client.ListGroupMembers(ctx, group.ID)

	return groupMembership{
			groupID: group.ID,
			members: members,
		},
		trace.Wrap(err, "failed to list group members for group %q", group.ID)
}

type roleMembership struct {
	roleID          string
	members         []netiqclient.RoleAssignmentStatus
	mappedResources []netiqclient.ResourceRef
	parentRoles     []netiqclient.RoleRef
}

// collectRoleMemberships collects the role memberships for the given role.
// It returns a roleMembership struct containing the role ID, the list of role
// members, the list of mapped resources, and the list of parent roles.
func (s *Service) collectRoleMemberships(ctx context.Context, role netiqclient.Role) (roleMembership, error) {
	members, err := s.client.ListRoleMembers(ctx, role.ID)
	if err != nil {
		return roleMembership{}, trace.Wrap(err, "failed to get role members for role %q", role.ID)
	}

	mappedResources, err := s.client.ListMappedResources(ctx, role.ID)
	if err != nil {
		return roleMembership{}, trace.Wrap(err, "failed to get role mapped resources for role %q", role.ID)
	}

	parentRoles, err := s.client.ListRoleParentRoles(ctx, role.ID)
	if err != nil {
		return roleMembership{}, trace.Wrap(err, "failed to get role parent roles for role %q", role.ID)
	}

	return roleMembership{
		roleID:          role.ID,
		members:         members,
		mappedResources: mappedResources,
		parentRoles:     parentRoles,
	}, nil
}

// convertUsers converts the list of users to a slice of NetIQUser.
func convertUsers(users []netiqclient.User) []*accessgraphv1alpha.NetIQUser {
	out := make([]*accessgraphv1alpha.NetIQUser, 0, len(users))
	for _, user := range users {
		out = append(out, &accessgraphv1alpha.NetIQUser{
			Id:         user.DN,
			Email:      user.Email,
			Name:       user.FullName,
			IsDisabled: user.IsDisabled,
		})
	}
	return out
}

// convertResources converts the list of resources to a slice of NetIQResource.
func convertResources(resources []netiqclient.Resource) []*accessgraphv1alpha.NetIQResource {
	out := make([]*accessgraphv1alpha.NetIQResource, 0, len(resources))
	for _, resource := range resources {
		out = append(out, &accessgraphv1alpha.NetIQResource{
			Id:          resource.ID,
			Name:        resource.Name,
			Description: resource.Description,
			Categories:  convertCategories(resource.Categories),
		})
	}
	return out
}

// convertCategories converts the list of categories to a slice of NetIQCategory.
func convertCategories(categories []netiqclient.Category) []*accessgraphv1alpha.NetIQCategory {
	out := make([]*accessgraphv1alpha.NetIQCategory, 0, len(categories))
	for _, category := range categories {
		out = append(out, &accessgraphv1alpha.NetIQCategory{
			Id:   category.ID,
			Name: category.Name,
		})
	}
	return out
}

// convertGroups converts the list of groups to a slice of NetIQGroup.
func convertGroups(groups []netiqclient.Group) []*accessgraphv1alpha.NetIQGroup {
	out := make([]*accessgraphv1alpha.NetIQGroup, 0, len(groups))
	for _, group := range groups {
		out = append(out, &accessgraphv1alpha.NetIQGroup{
			Id:          group.ID,
			Name:        group.Name,
			Description: group.Description,
		})
	}
	return out
}

// convertRoles converts the list of roles to a slice of NetIQRole.
func convertRoles(roles []netiqclient.Role) []*accessgraphv1alpha.NetIQRole {
	out := make([]*accessgraphv1alpha.NetIQRole, 0, len(roles))
	for _, role := range roles {
		out = append(out, &accessgraphv1alpha.NetIQRole{
			Id:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			Categories:  convertCategories(role.Categories),
			Level: &accessgraphv1alpha.NetIQRole_RoleLevel{
				Name:  role.RoleLevel.Name,
				Level: int32(role.RoleLevel.Level),
				Cn:    role.RoleLevel.Cn,
			},
		})
	}
	return out
}

// convertGroupMembers converts the map of group members to a slice of
// NetIQGroupMember.
func convertGroupMembers(groupID string, groupMembers []netiqclient.GroupMember) []*accessgraphv1alpha.NetIQGroupMember {
	out := make([]*accessgraphv1alpha.NetIQGroupMember, 0, len(groupMembers))
	for _, member := range groupMembers {
		out = append(out, &accessgraphv1alpha.NetIQGroupMember{
			GroupId:           groupID,
			UserId:            member.Dn,
			IsGroupAssignment: member.IsGroupAssignment,
		})
	}

	return out
}

// convertRoleMembers converts the map of role members to a slice of
// NetIQMemberAssignmentRef.
func convertRoleMembers(roleID string, roleMembers []netiqclient.RoleAssignmentStatus) []*accessgraphv1alpha.NetIQMemberAssignmentRef {
	out := make([]*accessgraphv1alpha.NetIQMemberAssignmentRef, 0)
	for _, member := range roleMembers {
		statusCode, _ := strconv.Atoi(member.StatusCode)
		out = append(out, &accessgraphv1alpha.NetIQMemberAssignmentRef{
			RoleId:                    roleID,
			Dn:                        member.RecipientDn,
			RecipientType:             recipientTypeToEnum(member.RecipientType),
			RecipientTypeSubcontainer: member.RecipientTypeSubContainer,
			StatusCode:                uint32(statusCode),
			StatusDisplay:             member.StatusDisplay,
			EffectiveDate:             dateToTime(member.EffectiveDate),
			ExpiryDate:                dateToTime(member.ExpiryDate),
			Description:               member.Description,
			Grant:                     member.Grant,
		})

	}
	return out
}

// dateToTime converts a date string to a timestamppb.Timestamp.
// The string is expected to be a Unix timestamp in seconds.
func dateToTime(date string) *timestamppb.Timestamp {
	if date == "" {
		return nil
	}

	t, err := strconv.ParseInt(date, 10, 64)
	if err != nil {
		slog.WarnContext(context.TODO(), "Failed to parse unix timestamp date.", "date", date, "error", err)
		return nil
	}

	return timestamppb.New(time.Unix(t, 0))
}

// recipientTypeToEnum converts a recipient type string to a RoleRecipientType.
func recipientTypeToEnum(recipientType string) accessgraphv1alpha.RoleRecipientType {
	switch {
	case strings.EqualFold(recipientType, "USER"):
		return accessgraphv1alpha.RoleRecipientType_ROLE_RECIPIENT_TYPE_USER
	case strings.EqualFold(recipientType, "GROUP"):
		return accessgraphv1alpha.RoleRecipientType_ROLE_RECIPIENT_TYPE_GROUP
	default:
		slog.WarnContext(context.TODO(), "Unknown recipient type.", "recipient_type", recipientType)
		return accessgraphv1alpha.RoleRecipientType_ROLE_RECIPIENT_TYPE_UNSPECIFIED
	}
}

// convertRoleMappedResources converts the map of role mapped resources to a
// slice of NetIQResourceAssignmentRef.
func convertRoleMappedResources(roleID string, roleMappedResources []netiqclient.ResourceRef) []*accessgraphv1alpha.NetIQResourceAssignmentRef {
	out := make([]*accessgraphv1alpha.NetIQResourceAssignmentRef, 0)

	for _, resource := range roleMappedResources {
		entitlements := make([]*accessgraphv1alpha.Entitlement, len(resource.Entitlements))
		for i, entitlement := range resource.Entitlements {
			entitlements[i] = &accessgraphv1alpha.Entitlement{
				Id:    entitlement.ID,
				Name:  entitlement.Name,
				Value: entitlement.Value,
			}
		}
		out = append(out, &accessgraphv1alpha.NetIQResourceAssignmentRef{
			RoleId:             roleID,
			ResourceId:         resource.ID,
			MappingDescription: resource.MappingDescription,
			StatusCode:         uint32(resource.Status),
			Entitlements:       entitlements,
		})
	}
	return out
}

// convertRoleParentRoles converts the map of role parent roles to a slice of
// NetIQRoleRef.
func convertRoleParentRoles(roleID string, roleParentRoles []netiqclient.RoleRef) []*accessgraphv1alpha.NetIQRoleRef {
	out := make([]*accessgraphv1alpha.NetIQRoleRef, 0)
	for _, role := range roleParentRoles {
		out = append(out, &accessgraphv1alpha.NetIQRoleRef{
			ChildRoleId:        roleID,
			ParentRoleId:       role.ID,
			Level:              int32(role.Level),
			RequestDescription: role.RequestDescription,
		})
	}
	return out
}

type resources struct {
	// users is the list of users.
	users []*accessgraphv1alpha.NetIQUser
	// resources is the list of resources.
	resources []*accessgraphv1alpha.NetIQResource
	// groups is the list of groups.
	groups []*accessgraphv1alpha.NetIQGroup
	// groupMembers is the list of group members.
	groupMembers []*accessgraphv1alpha.NetIQGroupMember
	// roles is the list of roles.
	roles []*accessgraphv1alpha.NetIQRole
	// roleMembers is the list of role members.
	roleMembers []*accessgraphv1alpha.NetIQMemberAssignmentRef
	// roleMappedResources is the list of role mapped resources.
	mappedResources []*accessgraphv1alpha.NetIQResourceAssignmentRef
	// roleParentRoles is the list of role parent roles.
	parentRoles []*accessgraphv1alpha.NetIQRoleRef
}
