package identitycenter

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/api/types/header"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/lib/services"
)

// maybeImportGroupAndGroupMembers checks for plugin group import status and triggers import
// if the import status is not set to DONE.
func (svc *Service) maybeImportGroupAndGroupMembers(ctx context.Context) error {
	plugin, err := svc.pluginsService.GetPlugin(ctx, types.PluginTypeAWSIdentityCenter, false /* withSecrets */)
	if err != nil {
		return trace.Wrap(err)
	}

	if requiresImport(plugin.GetStatus().GetAwsIc()) {
		svc.log.InfoContext(ctx, "Importing Identity Center groups and group members...")
		if err := svc.importAndEmitStatus(ctx); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func requiresImport(pluginStatus *types.PluginAWSICStatusV1) bool {
	// pluginStatus can be empty if the plugin status has never been updated
	// (a brand new plugin or the initial status is yet to be emitted/reconciled).
	if pluginStatus == nil {
		return true
	}
	// pluginStatus.GroupImportStatus can be nil if group import has never been run.
	if pluginStatus.GroupImportStatus == nil {
		return true
	}
	if pluginStatus.GroupImportStatus.StatusCode != types.AWSICGroupImportStatusCode_DONE {
		return true
	}

	return false
}

func (svc *Service) importAndEmitStatus(ctx context.Context) error {
	if err := svc.startGroupsAndGroupMembersImport(ctx); err != nil {
		// log error here and let the status emitter return error to the user.
		svc.log.ErrorContext(ctx, "failed groups or group members imports", "error", err)
		return svc.emitImportStatus(ctx, &types.AWSICGroupImportStatus{
			StatusCode:   types.AWSICGroupImportStatusCode_FAILED,
			ErrorMessage: err.Error(),
		})
	}

	return svc.emitImportStatus(ctx, &types.AWSICGroupImportStatus{
		StatusCode: types.AWSICGroupImportStatusCode_DONE,
	})
}

func (svc *Service) emitImportStatus(ctx context.Context, status *types.AWSICGroupImportStatus) error {
	if err := svc.pluginStatusSink.Emit(ctx, &types.PluginStatusV1{
		Details: &types.PluginStatusV1_AwsIc{
			AwsIc: &types.PluginAWSICStatusV1{
				GroupImportStatus: status,
			},
		},
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// startGroupsAndGroupMembersImport imports Identity Center groups and group members as
// Teleport Access List and Access List members respectively.
// It also assigns roles to Access List based on account and permission set assignment
// to the corresponding Identity Center group in AWS.
//   - It creates Access List for each Identity Center groups. All such Access List are
//     labeled with OriginAWSIdentityCenter.
//   - For an existing Access List with OriginAWSIdentityCenter origin label, the reconciler
//     replaces existing one, including roles and members with a new data freshly imported from
//     Identity Center.
//   - It imports Access List members based on Identity Center group members. Group members whose
//     user account does not exist in Teleport are filtered.
//
// Important: startGroupsAndGroupMembersImport should be run only once during an initial plugin start
// and if the group import has not successfully been imported previously. Because we want Teleport to
// be the source of truth for Identity Center group and group members, repeated imports voids that.
func (s *Service) startGroupsAndGroupMembersImport(ctx context.Context) error {
	existingList, err := accessListFromTeleport(ctx, s.accessListSvc)
	if err != nil {
		return trace.Wrap(err)
	}
	newList, err := s.accessListFromICGroups(ctx, toAclOwner(s.importConfig.AccessListDefaultOwners))
	if err != nil {
		return trace.Wrap(err)
	}
	accessListReconciler, err := accessListReconciler(s.accessListSvc, existingList, newList)
	if err != nil {
		return trace.Wrap(err)
	}

	existingMembers, err := accessListMembersFromTeleport(ctx, accessListNames(existingList), s.accessListSvc)
	if err != nil {
		return trace.Wrap(err)
	}
	newMembers, err := s.accessListMembersFromIC(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	memberReconciler, err := accessListMembersReconciler(s.accessListSvc, existingMembers, newMembers)
	if err != nil {
		return trace.Wrap(err)
	}

	err = trace.NewAggregate(
		accessListReconciler.Reconcile(ctx),
		memberReconciler.Reconcile(ctx),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// accessListFromICGroups returns a map of Access List for each corresponding Identity Center group.
// For each account assignment in Identity Center, a role for that is assigned to the Access List.
// Role name is configured as "<permission_set_name>-on-<account_name>". This is the same format used
// by the identity Center role<>permission assignment reconciler.
func (s *Service) accessListFromICGroups(ctx context.Context, defaultOwners []accesslist.Owner) (map[string]*accesslist.AccessList, error) {
	groupsWithAssignments, err := s.icClient.ListGroupsWithAccountAndPermAssignment(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	permSets, err := s.icClient.ListPermissionSets(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accounts, err := s.icClient.ListAccounts(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	groupsAssignmentWithAccountAndPermissionSetName := groupAccountAndPermAssigments(
		ctx,
		groupsWithAssignments,
		icsdk.ToAccountMap(accounts),
		icsdk.ToPermissionSetMap(permSets),
		s.log,
	)

	out := map[string]*accesslist.AccessList{}
	for _, g := range groupsAssignmentWithAccountAndPermissionSetName {
		var aclRoles []string
		for _, ga := range g.assignments {
			aclRoles = append(aclRoles, getImportedRoleName(ga.permissionSetName, ga.accountName))
		}

		acl, err := accesslist.NewAccessList(
			header.Metadata{
				Name: g.ID,
			},
			accesslist.Spec{
				Title:  g.DisplayName,
				Owners: defaultOwners,
				Grants: accesslist.Grants{
					Roles: aclRoles,
				},
			},
		)
		if err != nil {
			// TODO(sshah): return error or swallow the error and log here?
			return nil, trace.Wrap(err)
		}
		acl.SetOrigin(common.OriginAWSIdentityCenter)
		out[acl.GetName()] = acl
	}

	return out, nil
}

// groupWithAccountAndPermAssignment represents Identity Center group with
// permission assignment.
type groupWithAccountAndPermAssignment struct {
	*icsdk.Group
	// assignments is a list of account name and permission set
	// assigned to a group.
	assignments []*accountAndPermAssignment
}

// accountAndPermAssignment represents permission assignment (permission set + account).
type accountAndPermAssignment struct {
	// accountName is the name of an assigned account.
	accountName string
	// permissionSetName is the name of the assigned permission set.
	permissionSetName string
}

// groupAccountAndPermAssigments transforms icsdk GroupWithAssignment to groupWithAccountAndPermAssignment with enriched
// account name and permission set name.
func groupAccountAndPermAssigments(
	ctx context.Context,
	in []*icsdk.GroupWithAssignment,
	accountMap icsdk.AccountMap,
	permSetMap icsdk.PermissionSetMap,
	log *slog.Logger,
) []*groupWithAccountAndPermAssignment {
	out := make([]*groupWithAccountAndPermAssignment, 0, len(in))
	for _, g := range in {
		out = append(out, &groupWithAccountAndPermAssignment{
			Group: &icsdk.Group{
				DisplayName: g.DisplayName,
				ID:          g.ID,
			},
			assignments: accountAndPermAssignments(ctx, g.DisplayName, g.Assignments, accountMap, permSetMap, log),
		})
	}
	return out
}

// accountAndPermAssignments transforms icsdk Assigment to accountAndPermAssignment
// with enriched account name and permission set name.
func accountAndPermAssignments(
	ctx context.Context,
	groupName string,
	assignment []*icsdk.Assigment,
	accountMap icsdk.AccountMap,
	permSetMap icsdk.PermissionSetMap,
	log *slog.Logger,
) []*accountAndPermAssignment {
	out := make([]*accountAndPermAssignment, 0, len(assignment))
	for _, a := range assignment {
		// we expect map entry for each assigned account ID and permission set ARN
		// but we still need to check in order to avoid panic.
		if _, ok := accountMap[a.AccountID]; !ok {
			log.InfoContext(ctx, "Account assignment not found", "group", groupName, "account_id", a.AccountID)
			continue
		}
		if _, ok := permSetMap[a.PermissionSetARN]; !ok {
			log.InfoContext(ctx, "Permission assignment not found", "group", groupName, "permission_set_arn", a.PermissionSetARN)
			continue
		}
		out = append(out, &accountAndPermAssignment{
			accountName:       accountMap[a.AccountID].Name,
			permissionSetName: permSetMap[a.PermissionSetARN].Name,
		})
	}
	return out
}

// accessListMembersFromIC returns new Acess List members for each group members from Identity Center.
// Members whose user account does not exist in Teleport are filtered.
func (s *Service) accessListMembersFromIC(ctx context.Context) (map[string]*accesslist.AccessListMember, error) {
	out := map[string]*accesslist.AccessListMember{}

	groupMembersFromIC, err := listGroupMembersFromIC(ctx, s.icClient)
	if err != nil {
		return nil, trace.Wrap(err, "listing group members from identity center")
	}

	teleportUsers, err := listTeleportUsers(ctx, s.usersSvc)
	if err != nil {
		return nil, trace.Wrap(err, "listing teleport user to filter Access List members.")
	}

	for _, gm := range groupMembersFromIC {
		for _, m := range gm.Members {
			if _, ok := teleportUsers[m]; !ok {
				continue
			}
			aclMember, err := newAccessListMember(gm.GroupID, m)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			out[memberMapKey(aclMember)] = aclMember
		}
	}

	return out, nil
}

// listGroupMembersFromIC returns Identity Center group members with their respective username.
func listGroupMembersFromIC(ctx context.Context, icClient icsdk.Client) ([]groupMembersWithUsername, error) {
	users, err := icClient.ListUsers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	usersMap := icsdk.ToUserMap(users)

	groupWithMembers, err := icClient.ListGroupsWithMembers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]groupMembersWithUsername, 0, len(groupWithMembers))
	for _, g := range groupWithMembers {
		out = append(out, groupMembersWithUsername{
			GroupID: g.ID,
			Members: usernamesFromMemberID(g.Members, usersMap),
		})
	}

	return out, nil
}

type groupMembersWithUsername struct {
	GroupID string
	Members []string
}

func usernamesFromMemberID(membersID []*icsdk.GroupMember, usermap icsdk.UserMap) []string {
	out := make([]string, 0, len(membersID))
	for _, m := range membersID {
		out = append(out, usermap[m.MemberID].UserName)
	}
	return out
}

func newAccessListMember(aclName, member string) (*accesslist.AccessListMember, error) {
	alm, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: member,
		},
		accesslist.AccessListMemberSpec{
			AccessList: aclName,
			Name:       member,
			Joined:     time.Now().UTC(),
			AddedBy:    teleport.UserSystem,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return alm, nil
}

// accessListReconciler reconciles imported Identity Center group as an Access List.
// It is only invoked once from the importGroupsAndGroupMembers method.
func accessListReconciler(
	service services.AccessLists,
	listInTeleport map[string]*accesslist.AccessList,
	newListFromIC map[string]*accesslist.AccessList,
) (*services.Reconciler[*accesslist.AccessList], error) {
	return services.NewReconciler(services.ReconcilerConfig[*accesslist.AccessList]{
		Matcher:             matchByOriginAWSIdentityCenterLabel[*accesslist.AccessList],
		GetCurrentResources: func() map[string]*accesslist.AccessList { return listInTeleport },
		GetNewResources:     func() map[string]*accesslist.AccessList { return newListFromIC },
		OnCreate: func(ctx context.Context, al *accesslist.AccessList) error {
			_, err := service.UpsertAccessList(ctx, al)
			return trace.Wrap(err)
		},
		OnUpdate: func(ctx context.Context, incoming *accesslist.AccessList, existing *accesslist.AccessList) error {
			_, err := service.UpsertAccessList(ctx, incoming)
			return trace.Wrap(err)
		},
		OnDelete: func(ctx context.Context, al *accesslist.AccessList) error {
			err := service.DeleteAccessList(ctx, al.GetName())
			return trace.Wrap(err)
		},
	})
}

// accessListMembersReconciler reconciles imported Identity Center group member as an Access List member.
// It is only invoked once from the importGroupsAndGroupMembers method.
func accessListMembersReconciler(
	service services.AccessLists,
	listMembersInTeleport map[string]*accesslist.AccessListMember,
	newListMembersFromIC map[string]*accesslist.AccessListMember,
) (*services.Reconciler[*accesslist.AccessListMember], error) {
	return services.NewReconciler(services.ReconcilerConfig[*accesslist.AccessListMember]{
		Matcher:             wildcardMatcher[*accesslist.AccessListMember],
		GetCurrentResources: func() map[string]*accesslist.AccessListMember { return listMembersInTeleport },
		GetNewResources:     func() map[string]*accesslist.AccessListMember { return newListMembersFromIC },
		OnCreate: func(ctx context.Context, m *accesslist.AccessListMember) error {
			_, err := service.UpsertAccessListMember(ctx, m)
			return trace.Wrap(err)
		},
		OnUpdate: func(ctx context.Context, incoming *accesslist.AccessListMember, existing *accesslist.AccessListMember) error {
			_, err := service.UpsertAccessListMember(ctx, incoming)
			return trace.Wrap(err)
		},
		OnDelete: func(ctx context.Context, m *accesslist.AccessListMember) error {
			err := service.DeleteAccessListMember(ctx, m.Spec.AccessList, m.Spec.Name)
			if trace.IsNotFound(err) {
				return nil
			}
			return trace.Wrap(err)
		},
	})
}

func toAclOwner(in []string) []accesslist.Owner {
	var out []accesslist.Owner
	for _, n := range in {
		if n != "" {
			out = append(out, accesslist.Owner{
				Name: n,
			})
		}

	}
	return out
}

func accessListNames(in map[string]*accesslist.AccessList) []string {
	out := make([]string, 0, len(in))
	for _, list := range in {
		out = append(out, list.GetName())
	}

	return out
}

// wildcardMatcher should only be used to provide a default true matcher.
func wildcardMatcher[T types.Resource](resource T) bool {
	return true
}

// matchByOriginAWSIdentityCenterLabel matches resource metadata label to
// contain origin value of OriginAWSIdentityCenter.
func matchByOriginAWSIdentityCenterLabel[T types.Resource](resource T) bool {
	origin, ok := resource.GetMetadata().Labels[types.OriginLabel]
	return ok && origin == common.OriginAWSIdentityCenter
}
