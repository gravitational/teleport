package entraid

import (
	"context"
	"log/slog"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/clientutils"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/teleport/lib/services"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

type accessListWithMembers struct {
	*accesslist.AccessList
	Members []*accesslist.AccessListMember
}

// GetKind returns a fake resource kind printed in the [services.Reconciler] logs.
func (a *accessListWithMembers) GetKind() string {
	return types.KindAccessList + "+" + types.KindAccessListMember
}

func (r *DirectoryReconciler) reconcileAccessLists(ctx context.Context,
	usersByEntraID map[entraUniqueID]types.User,
	groupsMap map[string]*msgraph.Group,
	groupMembersMap map[string][]msgraph.GroupMember,
	teleportAccessListsWithMembersMap map[string]*accessListWithMembers,
) error {
	entraAccessListWithMembersMap := convertEntraAccessListsWithMembers(ctx, usersByEntraID, groupsMap, groupMembersMap, r.tenantID, r.defaultOwners)

	// It's crucial to sort the members for the CompareResources func in the Reconciler.
	sortMembers(teleportAccessListsWithMembersMap)
	sortMembers(entraAccessListWithMembersMap)

	preserveFields(entraAccessListWithMembersMap, teleportAccessListsWithMembersMap)

	var alsWithNestedMembers []*accessListWithMembers
	onUpsert := func(ctx context.Context, a *accessListWithMembers) error {
		hasNestedMember := slices.ContainsFunc(a.Members, func(m *accesslist.AccessListMember) bool {
			return m.Spec.MembershipKind == accesslist.MembershipKindList
		})

		// If access list has a nested member, don't upsert it with members, because some
		// of the member list may not be provisioned yet. It will be upserted with members
		// on a second pass when all access lists are already upserted.
		if hasNestedMember {
			if _, err := r.accessListSvc.UpsertAccessList(ctx, a.AccessList); err != nil {
				return trace.Wrap(err)
			}
			alsWithNestedMembers = append(alsWithNestedMembers, a)
			return nil
		}
		_, _, err := r.accessListSvc.UpsertAccessListWithMembers(ctx, a.AccessList, a.Members)
		return trace.Wrap(err)
	}

	alReconciler, err := services.NewReconciler(services.ReconcilerConfig[*accessListWithMembers]{
		Matcher:             func(a *accessListWithMembers) bool { return matchByLabel(a.AccessList) },
		GetCurrentResources: func() map[string]*accessListWithMembers { return teleportAccessListsWithMembersMap },
		GetNewResources:     func() map[string]*accessListWithMembers { return entraAccessListWithMembersMap },
		OnCreate: func(ctx context.Context, a *accessListWithMembers) error {
			return trace.Wrap(onUpsert(ctx, a))
		},
		OnUpdate: func(ctx context.Context, incoming, existing *accessListWithMembers) error {
			return trace.Wrap(onUpsert(ctx, incoming))
		},
		OnDelete: func(ctx context.Context, a *accessListWithMembers) error {
			// DeleteAccessList will also delete its members.
			err := r.accessListSvc.DeleteAccessList(ctx, a.AccessList.GetName())
			return trace.Wrap(err)
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	var reconcileErrs []error

	if err := alReconciler.Reconcile(ctx); err != nil {
		reconcileErrs = append(reconcileErrs, trace.Wrap(err))
	}

	// Now all access lists are upserted, we can do a second pass and upsert all members for
	// access lists with nested members.
	// BTW, there is no need to do the same thing for potential nested owners as all owners are
	// overwritten with r.defaultOwners.
	for _, a := range alsWithNestedMembers {
		if _, _, err := r.accessListSvc.UpsertAccessListWithMembers(ctx, a.AccessList, a.Members); err != nil {
			reconcileErrs = append(reconcileErrs, trace.Wrap(err))
		}
	}

	if len(reconcileErrs) > 0 {
		return trace.NewAggregate(reconcileErrs...)
	}

	r.importedGroups = len(entraAccessListWithMembersMap)
	return nil
}

func listTeleportAccessListsWithMembers(ctx context.Context, svc accessListAccessPoint) (map[string]*accessListWithMembers, error) {
	aclsWithMembersMap := make(map[string]*accessListWithMembers)

	for al, err := range clientutils.Resources(ctx, svc.ListAccessLists) {
		if err != nil {
			return nil, trace.Wrap(err, "listing access lists")
		}
		if !matchByLabel(al) {
			continue
		}
		listMembersFn := func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
			r, token, err := svc.ListAccessListMembers(ctx, al.GetName(), pageSize, pageToken)
			return r, token, trace.Wrap(err)
		}
		var members []*accesslist.AccessListMember
		for m, err := range clientutils.Resources(ctx, listMembersFn) {
			if err != nil {
				return nil, trace.Wrap(err, "listing access list %q members", al.GetName())
			}
			members = append(members, m)
		}
		aclsWithMembersMap[al.GetName()] = &accessListWithMembers{
			AccessList: al,
			Members:    members,
		}
	}

	return aclsWithMembersMap, nil
}

func convertEntraAccessListsWithMembers(
	ctx context.Context,
	usersByEntraID map[entraUniqueID]types.User,
	groupsMap map[string]*msgraph.Group,
	groupMembersMap map[string][]msgraph.GroupMember,
	tenantID string,
	defaultOwners []accesslist.Owner,
) map[string]*accessListWithMembers {
	aclsWithMembersMap := make(map[string]*accessListWithMembers)
	accessListsById := make(map[entraUniqueID]*accesslist.AccessList)

	for _, g := range groupsMap {
		entraUniqueID, al, err := convertGroup(g, tenantID, defaultOwners)
		if err != nil {
			slog.ErrorContext(ctx, "failed to convert Entra ID group to Teleport access list", "error", err)
			continue
		}
		aclsWithMembersMap[al.GetName()] = &accessListWithMembers{AccessList: al}
		accessListsById[entraUniqueID] = al
	}

	for entraUniqueID, accessList := range accessListsById {
		var members []*accesslist.AccessListMember
		for _, member := range groupMembersMap[string(entraUniqueID)] {
			m, err := convertGroupMember(ctx, member, accessList, usersByEntraID, accessListsById)
			if err != nil {
				id := strval(member.GetID())
				slog.WarnContext(ctx, "error while converting group member", "member", id, "error", err)
				continue
			}
			if m == nil {
				slog.WarnContext(ctx, "unsupported group member, skipping")
				continue
			}
			members = append(members, m)
		}
		aclsWithMembersMap[accessList.GetName()].Members = members
	}

	return aclsWithMembersMap
}

func convertGroup(in *msgraph.Group, tenantID string, defaultOwners []accesslist.Owner) (entraUniqueID, *accesslist.AccessList, error) {
	if in == nil {
		return "", nil, trace.BadParameter("provided Entra ID group is nil")
	}
	if in.DisplayName == nil {
		return "", nil, trace.BadParameter("expected Entra ID group to have a non-empty display name")
	}
	displayName := *in.DisplayName
	if in.ID == nil {
		return "", nil, trace.BadParameter("expected Entra ID group to have a non-empty ID")
	}
	id := *in.ID

	out, err := accesslist.NewAccessList(
		header.Metadata{
			Name: accessListName(displayName, id),
		},
		accesslist.Spec{
			Title:  displayName,
			Owners: defaultOwners,
			Grants: accesslist.Grants{
				Traits: trait.Traits{
					eteleport.EntraMemberOfGroupTrait: {id},
				},
			},
		},
	)
	if err != nil {
		return "", nil, trace.Wrap(err)
	}
	out.SetStaticLabels(map[string]string{
		types.EntraTenantIDLabel:    tenantID,
		types.EntraUniqueIDLabel:    id,
		types.EntraDisplayNameLabel: displayName,
	})
	out.SetOrigin(types.OriginEntraID)
	return entraUniqueID(id), out, nil
}

// convertGroupMember converts an Entra group member to an AccessListMember.
// Error is returned on unexpected conditions, indicating programmer error (e.g. validation of AccessListMember fails).
// On non fatal errors, e.g. an unsupported member type, a warning is logged and (nil, nil) is returned.
func convertGroupMember(ctx context.Context,
	in msgraph.GroupMember,
	al *accesslist.AccessList,
	entraUsersByID map[entraUniqueID]types.User,
	accesslistByEntraId map[entraUniqueID]*accesslist.AccessList,
) (*accesslist.AccessListMember, error) {
	if in.GetID() == nil {
		return nil, trace.BadParameter("expected Entra ID user to have a non-empty unique ID")
	}
	id := *in.GetID()

	switch in.(type) {
	case *msgraph.User:
		teleportUser, ok := entraUsersByID[entraUniqueID(id)]
		if !ok {
			slog.WarnContext(ctx, "no teleport user found for Entra unique ID", "id", id)
			return nil, nil
		}
		alm, err := accesslist.NewAccessListMember(
			header.Metadata{
				Name: teleportUser.GetName(),
			},
			accesslist.AccessListMemberSpec{
				AccessList:     al.GetName(),
				Name:           teleportUser.GetName(),
				Joined:         time.Now().UTC(),
				AddedBy:        teleport.UserSystem,
				MembershipKind: accesslistv1.MembershipKind_MEMBERSHIP_KIND_USER.String(),
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		alm.SetOrigin(types.OriginEntraID)
		return alm, nil

	case *msgraph.Group:
		accessList, ok := accesslistByEntraId[entraUniqueID(id)]
		if !ok {
			slog.WarnContext(ctx, "No access list found for Entra unique ID", "id", id)
			return nil, nil
		}
		alm, err := accesslist.NewAccessListMember(
			header.Metadata{
				Name: accessList.GetName(),
			},
			accesslist.AccessListMemberSpec{
				AccessList:     al.GetName(),
				Name:           accessList.GetName(),
				Joined:         time.Now().UTC(),
				AddedBy:        teleport.UserSystem,
				MembershipKind: accesslistv1.MembershipKind_MEMBERSHIP_KIND_LIST.String(),
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		alm.SetOrigin(types.OriginEntraID)
		return alm, nil

	default:
		slog.WarnContext(ctx, "entra group member is not of a supported type: ", "directory_object", in, "type", logutils.TypeAttr(in))
		return nil, nil
	}
}

// uuidNamespace is the namespace used for generating UUIDs for access lists.
// It is a UUID derived from the string "entraid".
var uuidNamespace = uuid.NewSHA1(uuid.Nil, []byte("entraid"))

func accessListName(displayName string, id string) string {
	p := path.Join(id, displayName)
	// generate a UUID from the path to ensure uniqueness
	// and to avoid collisions with other access lists.
	// This is necessary because access list names are used as keys in the backend
	// and must be unique and deterministic.
	return uuid.NewSHA1(uuidNamespace, []byte(p)).String()
}

func listEntraGroups(ctx context.Context, graphClient GraphClient, filterMatches func(g *msgraph.Group) bool) (map[string]*msgraph.Group, error) {
	isValidGroup := func(g *msgraph.Group) bool {
		return g != nil && g.ID != nil && g.DisplayName != nil
	}
	result := map[string]*msgraph.Group{}
	err := graphClient.IterateGroups(ctx, func(g *msgraph.Group) bool {
		if isValidGroup(g) && filterMatches(g) {
			result[*g.ID] = g
		}

		// defaults to true so the iteration continues.
		return true
	})
	return result, trace.Wrap(err)
}

func listEntraGroupsMembers(ctx context.Context, graphClient GraphClient, groups map[string]*msgraph.Group) (map[string][]msgraph.GroupMember, error) {
	// membersPageSize is the maximum number of members to fetch per page.
	// https://learn.microsoft.com/en-us/graph/api/group-list-members?view=graph-rest-1.0&tabs=http#http-request
	// We don't want to send 9 requests to fetch 900 members where 999 is max page size supported by API
	const membersPageSize = 300

	result := make(map[string][]msgraph.GroupMember, len(groups))
	var mu sync.Mutex

	// TODO(smallinsky) move to static goroutine workers to not allocate space for each goroutine.
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(getParallelReqCount(len(groups)))
	for id, group := range groups {
		id, gid := id, *group.ID
		g.Go(func() error {
			var members []msgraph.GroupMember
			if err := graphClient.IterateGroupMembers(ctx, gid, func(m msgraph.GroupMember) bool {
				members = append(members, m)
				return true
			}, msgraph.WithTop(membersPageSize)); err != nil {
				return trace.Wrap(err)
			}

			mu.Lock()
			result[id] = members
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}
	return result, nil
}

// getParallelReqCount returns the number of parallel requests to use based on the number of groups.
// We want to balance and not run 80 parallel request for 90 groups.
// But with large dataset like 10k we want to have enough parallelism to not take hours to fetch all members.
func getParallelReqCount(numGroups int) int {
	if numGroups < 1000 {
		return 10
	}
	// With 30k groups and 100 members assigned per group it takes around 3-4 minutes to fetch all members with 70 parallel requests.
	return 70
}

func unwindGroupMembership(groups map[string]*msgraph.Group, groupMembers map[string][]msgraph.GroupMember) map[string][]string {
	// result map to hold the membership paths for each group.
	result := make(map[string][]string)
	// visited tracks groups in the current path to avoid cycles.
	visited := make(map[string]bool)

	var collectPaths func(groupID string) []string
	collectPaths = func(groupID string) []string {
		// if the path for this group is already computed, return it.
		if path, exists := result[groupID]; exists {
			return path
		}

		// if the group is already visited in the current path, it means there is a cycle.
		if visited[groupID] {
			return []string{groupID} // Break the cycle by not proceeding further in this branch
		}

		visited[groupID] = true

		// path always starts with the current group.
		path := []string{groupID}

		// Traverse each member of the group.
		for _, member := range groupMembers[groupID] {
			if nestedGroup, ok := member.(*msgraph.Group); ok {
				// Skip Office 365 groups, we only care about security groups.
				if nestedGroup.IsOffice365Group() {
					continue
				}
				// recursively collect the path for nested groups.
				nestedPath := collectPaths(*nestedGroup.ID)
				path = append(path, nestedPath...)
			}
		}

		// store the computed path in result to avoid redundant calculations
		result[groupID] = path

		// unmark this group as visited after recursion completes
		visited[groupID] = false
		return path
	}

	// Collect the membership paths for each group.
	for groupID, group := range groups {
		// Skip Office 365 groups, we only care about security groups.
		if group.IsOffice365Group() {
			continue
		}
		collectPaths(groupID)
	}

	// Convert the result to a map where each group ID maps to a list of group IDs
	// that a user member of that group is automatically a member of.
	groupToMembership := make(map[string][]string)
	for childGroup, v := range result {
		for _, parentGroup := range v {
			groupToMembership[parentGroup] = append(groupToMembership[parentGroup], childGroup)
		}
	}
	for k, v := range groupToMembership {
		groupToMembership[k] = utils.Deduplicate(v)
	}
	return groupToMembership
}

func sortMembers(m map[string]*accessListWithMembers) {
	for _, a := range m {
		slices.SortFunc(a.Members, func(x, y *accesslist.AccessListMember) int {
			return strings.Compare(x.GetName(), y.GetName())
		})
	}
}

type memberKey struct {
	Name       string
	AccessList string
}

func newMemberKey(m *accesslist.AccessListMember) memberKey {
	return memberKey{
		Name:       m.GetName(),
		AccessList: m.Spec.AccessList,
	}
}

func preserveFields(dst, src map[string]*accessListWithMembers) {
	for k, dst := range dst {
		if src, ok := src[k]; ok {
			preserveAccessListFields(dst.AccessList, src.AccessList)
		}
	}

	dstMembersMap := make(map[memberKey]*accesslist.AccessListMember)
	for _, a := range dst {
		for _, m := range a.Members {
			dstMembersMap[newMemberKey(m)] = m
		}
	}
	for _, a := range src {
		for _, src := range a.Members {
			if dst, ok := dstMembersMap[newMemberKey(src)]; ok {
				preserveAccessListMemberFields(dst, src)
			}
		}
	}
}

func preserveAccessListFields(dst, src *accesslist.AccessList) {
	dst.Status = src.Status
	dst.Metadata.Revision = src.Metadata.Revision
	dst.Spec.Audit = src.Spec.Audit
	dst.Spec.Description = src.Spec.Description

	dst.Spec.Grants.Roles = src.Spec.Grants.Roles
	for k, v := range src.Spec.Grants.Traits {
		dstVal, ok := dst.Spec.Grants.Traits[k]
		if !ok {
			dst.Spec.Grants.Traits[k] = v
			continue
		}
		dst.Spec.Grants.Traits[k] = utils.Deduplicate(append(dstVal, v...))
	}
}

func preserveAccessListMemberFields(dst, src *accesslist.AccessListMember) {
	dst.Spec.Joined = src.Spec.Joined
}

func strval(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
