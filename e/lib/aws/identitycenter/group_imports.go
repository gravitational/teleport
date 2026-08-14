package identitycenter

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/header"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/e/lib/provisioning"
	icfilters "github.com/gravitational/teleport/lib/aws/identitycenter/filters"
	"github.com/gravitational/teleport/lib/services"
)

// maybeImportGroupAndGroupMembers checks for plugin group import status and triggers import
// if the import status is not set to DONE. The identity center service must bail out
// if this method returns an error. Otherwise it risks provisioning incorrect group data to AWS.
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

// importAndEmitStatus runs group import and reports import status to plugin status sink.
// Should always return an error if group import fails or if successful plugin status emission fails.
func (svc *Service) importAndEmitStatus(ctx context.Context) error {
	if importErr := svc.startGroupsAndGroupMembersImport(ctx); importErr != nil {
		svc.log.ErrorContext(ctx, "Failed to import groups or group members", "error", importErr)
		if err := svc.emitImportStatus(ctx, &types.AWSICGroupImportStatus{
			StatusCode:   types.AWSICGroupImportStatusCode_FAILED,
			ErrorMessage: importErr.Error(),
		}); err != nil {
			svc.log.ErrorContext(ctx, "Failed to emit group import failed status", "error", err)
		}
		return trace.Wrap(importErr)
	}

	if err := svc.emitImportStatus(ctx, &types.AWSICGroupImportStatus{
		StatusCode: types.AWSICGroupImportStatusCode_DONE,
	}); err != nil {
		svc.log.ErrorContext(ctx, "Group import succeeded but plugin status update failed", "error", err)
		return trace.Wrap(err)
	}

	return nil
}

func (svc *Service) emitImportStatus(ctx context.Context, importStatus *types.AWSICGroupImportStatus) error {
	pluginStatusCode := types.PluginStatusCode_RUNNING
	errorMessage := ""
	if importStatus.StatusCode == types.AWSICGroupImportStatusCode_FAILED {
		pluginStatusCode = types.PluginStatusCode_OTHER_ERROR
		errorMessage = "AWS IAM Identity Center groups import failed. Please see Auth log for more details."
	}
	if err := svc.pluginStatusSink.Emit(ctx, &types.PluginStatusV1{
		Code:         pluginStatusCode,
		ErrorMessage: errorMessage,
		Details: &types.PluginStatusV1_AwsIc{
			AwsIc: &types.PluginAWSICStatusV1{
				GroupImportStatus: importStatus,
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
func (svc *Service) startGroupsAndGroupMembersImport(ctx context.Context) error {
	existingList, err := ListICOriginatedAccessLists(ctx, svc.accessListSvcCache)
	if err != nil {
		return trace.Wrap(err)
	}
	newList, err := svc.accessListFromICGroups(ctx, toAclOwner(svc.importConfig.AccessListDefaultOwners))
	if err != nil {
		return trace.Wrap(err)
	}
	accessListReconciler, err := accessListReconciler(svc.accessListSvc, existingList, newList, svc.provisioner, svc.log)
	if err != nil {
		return trace.Wrap(err)
	}

	existingMembers, err := accessListMembersFromTeleport(ctx, accessListNames(existingList), svc.accessListSvcCache)
	if err != nil {
		return trace.Wrap(err)
	}
	newMembers, err := svc.accessListMembersFromIC(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	memberReconciler, err := accessListMembersReconciler(svc.accessListSvc, existingMembers, newMembers)
	if err != nil {
		return trace.Wrap(err)
	}

	if err = trace.NewAggregate(
		accessListReconciler.Reconcile(ctx),
		memberReconciler.Reconcile(ctx),
	); err != nil {
		svc.emitSyncEvent(ctx, &apievents.AWSICResourceSync{
			TotalUserGroups: int32(len(newList)),
			Status: apievents.Status{
				UserMessage: "User groups synchronization failed",
			},
		}, false /* failed */)
		return trace.Wrap(err)
	}

	svc.emitSyncEvent(ctx, &apievents.AWSICResourceSync{
		TotalUserGroups: int32(len(newList)),
		Status: apievents.Status{
			UserMessage: "User groups imported and synced as Access List",
		},
	}, true /* success */)

	return nil
}

// accessListFromICGroups returns a map of Access List for each corresponding Identity Center group.
// For each account assignment in Identity Center, a role for that is assigned to the Access List.
// Role name is configured as "<permission_set_name>-on-<account_name>". This is the same format used
// by the identity Center role<>permission assignment reconciler.
func (svc *Service) accessListFromICGroups(ctx context.Context, defaultOwners []accesslist.Owner) (map[string]*accesslist.AccessList, error) {
	groupsWithAssignments, err := ListGroupsWithAccountAndPermAssignment(ctx, svc.icClient, WithGroupFilters(svc.importConfig.GroupSyncFilter))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	permSets, err := svc.icClient.ListPermissionSets(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accounts, err := svc.icClient.ListAccounts(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accounts = filterAccounts(svc.importConfig.AccountFilters, accounts)

	groupsAssignmentWithAccountAndPermissionSetName := groupAccountAndPermAssignments(
		ctx,
		groupsWithAssignments,
		icsdk.ToAccountMap(accounts),
		icsdk.ToPermissionSetMap(permSets),
		svc.log,
	)

	out := map[string]*accesslist.AccessList{}
	for _, g := range groupsAssignmentWithAccountAndPermissionSetName {
		var aclRoles []string
		for _, ga := range g.assignments {
			aclRoles = append(aclRoles, getImportedRoleName(ga.permissionSetName, ga.accountName, ga.accountID))
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
	// accountID is the ID of an assigned account
	accountID string
	// accountName is the name of an assigned account.
	accountName string
	// permissionSetName is the name of the assigned permission set.
	permissionSetName string
}

// groupAccountAndPermAssignments transforms icsdk GroupWithAssignment to groupWithAccountAndPermAssignment with enriched
// account name and permission set name.
func groupAccountAndPermAssignments(
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

// accountAndPermAssignments transforms icsdk Assignment to accountAndPermAssignment
// with enriched account name and permission set name.
func accountAndPermAssignments(
	ctx context.Context,
	groupName string,
	assignment []*icsdk.Assignment,
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
			accountID:         a.AccountID,
			accountName:       accountMap[a.AccountID].Name,
			permissionSetName: permSetMap[a.PermissionSetARN].Name,
		})
	}
	return out
}

// accessListMembersFromIC returns new Acess List members for each group members
// from Identity Center. Members whose user account does not exist in Teleport
// are filtered.
func (svc *Service) accessListMembersFromIC(ctx context.Context) (map[string]*accesslist.AccessListMember, error) {
	out := map[string]*accesslist.AccessListMember{}

	teleportUsers, err := listTeleportUsers(ctx, svc.usersSvc)
	if err != nil {
		return nil, trace.Wrap(err, "listing teleport user to filter Access List members.")
	}

	groupMembersFromIC, err := listGroupMembersFromIC(ctx, svc.icClient, WithGroupFilters(svc.importConfig.GroupSyncFilter))
	if err != nil {
		return nil, trace.Wrap(err, "listing group members from identity center")
	}

	for _, gm := range groupMembersFromIC {
		for _, m := range gm.Members {
			isICOriginated := false
			if _, ok := teleportUsers[m.UserName]; !ok {
				isICOriginated = true
			}
			aclMember, err := newAccessListMember(gm.GroupID, m, isICOriginated)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			out[memberMapKey(aclMember)] = aclMember
		}
	}

	return out, nil
}

// listGroupMembersFromIC returns Identity Center group members with their respective username.
func listGroupMembersFromIC(ctx context.Context, icClient icsdk.Client, options ...GroupQueryOption) ([]groupMembersWithIDAndUserName, error) {
	icUsers, err := icClient.ListUsers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	icUsersMap := icsdk.ToUserMap(icUsers)

	groupWithMembers, err := ListGroupsWithMembers(ctx, icClient, options...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := make([]groupMembersWithIDAndUserName, 0, len(groupWithMembers))
	for _, g := range groupWithMembers {
		out = append(out, groupMembersWithIDAndUserName{
			GroupID: g.ID,
			Members: memberWithIDAndUsername(g.Members, icUsersMap),
		})
	}

	return out, nil
}

type groupMembersWithIDAndUserName struct {
	GroupID string
	Members []member
}

type member struct {
	ID       string
	UserName string
}

func memberWithIDAndUsername(members []*icsdk.GroupMember, usermap icsdk.UserMap) []member {
	out := make([]member, 0, len(members))
	for _, m := range members {
		if user, ok := usermap[m.MemberID]; ok {
			out = append(out, member{
				ID:       m.MemberID,
				UserName: user.UserName,
			})
		}
	}
	return out
}

func newAccessListMember(aclName string, member member, isICOriginated bool) (*accesslist.AccessListMember, error) {
	alm, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: member.UserName,
		},
		accesslist.AccessListMemberSpec{
			AccessList:     aclName,
			Name:           member.UserName,
			Joined:         time.Now().UTC(),
			AddedBy:        teleport.UserSystem,
			MembershipKind: accesslistv1.MembershipKind_MEMBERSHIP_KIND_USER.String(),
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if isICOriginated {
		alm.SetOrigin(common.OriginAWSIdentityCenter)
		alm.Metadata.Labels[provisioning.ExternalIDLabel.String()] = member.ID
	}

	return alm, nil
}

// accessListReconciler reconciles imported Identity Center group as an Access List.
// It is only invoked once from the importGroupsAndGroupMembers method.
func accessListReconciler(
	service services.AccessLists,
	listInTeleport map[string]*accesslist.AccessList,
	newListFromIC map[string]*accesslist.AccessList,
	scimProvisioner *provisioning.Service,
	logger *slog.Logger,
) (*services.Reconciler[*accesslist.AccessList], error) {
	onDelete := func(ctx context.Context, al *accesslist.AccessList) error {
		// Before we actually do anything here, we need to mark the access
		// list as "condemned" by the import service, meaning that they
		// should not be de-provisioned downstream by the provisioning service
		err := scimProvisioner.SetAccessListStateLabel(ctx, al.GetName(), principalDeleteLabel, principalDeleteModeTeleportOnly)
		if err != nil && !trace.IsNotFound(err) {
			// it's possible that we're attempting to remove a list before it has
			// even been provisioned, so not finding the record and receiving a
			// NotFound error is a legitimate condition. Otherwise...
			return trace.Wrap(err)
		}
		err = service.DeleteAccessList(ctx, al.GetName())
		return trace.Wrap(err)
	}

	return services.NewReconciler(services.ReconcilerConfig[*accesslist.AccessList]{
		Matcher: matchByOriginAWSIdentityCenterLabel[*accesslist.AccessList],
		CompareResources: func(al1, al2 *accesslist.AccessList) int {
			return services.EqualFromBool(accesslist.EqualAccessLists(al1, al2, accesslist.WithIgnoreEphemeralFields()))
		},
		GetCurrentResources: func() map[string]*accesslist.AccessList { return listInTeleport },
		GetNewResources:     func() map[string]*accesslist.AccessList { return newListFromIC },
		Logger:              logger,
		OnCreate: func(ctx context.Context, al *accesslist.AccessList) error {
			_, err := service.UpsertAccessList(ctx, al)
			return trace.Wrap(err)
		},
		OnUpdate: func(ctx context.Context, incoming *accesslist.AccessList, _ *accesslist.AccessList) error {
			_, err := service.UpsertAccessList(ctx, incoming)
			return trace.Wrap(err)
		},
		OnDelete: onDelete,
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
		Matcher: wildcardMatcher[*accesslist.AccessListMember],
		CompareResources: func(alm1, alm2 *accesslist.AccessListMember) int {
			if alm1.Spec.Name == alm2.Spec.Name &&
				alm1.Spec.AccessList == alm2.Spec.AccessList {
				return services.Equal
			}

			return services.Different
		},
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
				MembershipKind:   accesslistv1.MembershipKind_MEMBERSHIP_KIND_USER.String(),
				IneligibleStatus: accesslistv1.IneligibleStatus_name[int32(accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE)],
				Name:             n,
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

func collectGroupQueryOptions(opts []GroupQueryOption) groupQuery {
	var query groupQuery
	for _, optFn := range opts {
		optFn(&query)
	}
	return query
}

type groupQuery struct {
	filters icfilters.Filters
}

// GroupQueryOption defines an option setter for customizing group queries
type GroupQueryOption func(*groupQuery)

// WithGroupFilters adds filters to the group query
func WithGroupFilters(f icfilters.Filters) GroupQueryOption {
	return func(q *groupQuery) {
		q.filters = f
	}
}

// ListGroupsWithMembers lists Identity Center user groups with its respective members.
func ListGroupsWithMembers(ctx context.Context, c icsdk.Client, options ...GroupQueryOption) ([]*icsdk.GroupWithMembers, error) {
	query := collectGroupQueryOptions(options)

	groups, err := c.ListGroups(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	groups = filterGroups(query.filters, groups)

	var out []*icsdk.GroupWithMembers
	for _, g := range groups {
		members, err := c.ListGroupMemberships(ctx, g.ID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, &icsdk.GroupWithMembers{
			Group:   g,
			Members: members,
		})
	}
	return out, nil
}

// ListGroupsWithAccountAndPermAssignment lists Identity Center groups with assigned accounts and permission sets.
func ListGroupsWithAccountAndPermAssignment(ctx context.Context, c icsdk.Client, options ...GroupQueryOption) ([]*icsdk.GroupWithAssignment, error) {
	query := collectGroupQueryOptions(options)

	groups, err := c.ListGroups(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	groups = filterGroups(query.filters, groups)

	out := make([]*icsdk.GroupWithAssignment, 0, len(groups))
	for _, g := range groups {
		assignments, err := c.ListGroupsAssignments(ctx, g.ID)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		out = append(out, &icsdk.GroupWithAssignment{
			Group:       g,
			Assignments: assignments,
		})
	}
	return out, nil
}
