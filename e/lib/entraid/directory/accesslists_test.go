package directory

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/accesslists"
	"github.com/gravitational/teleport/lib/msgraph/models"
)

func TestUnwindGroupMembership(t *testing.T) {
	tests := []struct {
		name         string
		groups       groupsByID
		groupMembers groupMembersByGroupID
		expected     map[string][]string
	}{
		{
			name:         "empty input",
			groups:       groupsByID{},
			groupMembers: groupMembersByGroupID{},
			expected:     map[string][]string{},
		},
		{
			name: "single group",
			groups: groupsByID{
				"group1": {
					DirectoryObject: models.DirectoryObject{
						ID: valToPTR("group1"),
					},
				},
			},
			groupMembers: groupMembersByGroupID{},
			expected: map[string][]string{
				"group1": {"group1"},
			},
		},
		{
			name: "groups with other groups as members",
			groups: groupsByID{
				"group1": {
					DirectoryObject: models.DirectoryObject{
						ID: valToPTR("group1"),
					},
				},
				"group2": {
					DirectoryObject: models.DirectoryObject{
						ID: valToPTR("group2"),
					},
				},
				"group3": {
					DirectoryObject: models.DirectoryObject{
						ID: valToPTR("group3"),
					},
				},
			},
			groupMembers: groupMembersByGroupID{
				"group1": {
					&models.Group{
						DirectoryObject: models.DirectoryObject{
							ID: valToPTR("group2"),
						},
					},
					&models.Group{
						DirectoryObject: models.DirectoryObject{
							ID: valToPTR("group3"),
						},
					},
				},
			},
			expected: map[string][]string{
				"group1": {"group1"},
				"group2": {"group2", "group1"},
				"group3": {"group3", "group1"},
			},
		},
		{
			name: "complex group membership with 4 levels",
			groups: groupsByID{
				"group1": {
					DirectoryObject: models.DirectoryObject{
						ID: valToPTR("group1"),
					},
				},
				"group2": {
					DirectoryObject: models.DirectoryObject{
						ID: valToPTR("group2"),
					},
				},
				"group3": {
					DirectoryObject: models.DirectoryObject{
						ID: valToPTR("group3"),
					},
				},
				"group4": {
					DirectoryObject: models.DirectoryObject{
						ID: valToPTR("group4"),
					},
				},
				"group5": {
					DirectoryObject: models.DirectoryObject{
						ID: valToPTR("group5"),
					},
				},
				"group6": {
					DirectoryObject: models.DirectoryObject{
						ID: valToPTR("group6"),
					},
				},
			},
			groupMembers: groupMembersByGroupID{
				"group1": {
					&models.Group{
						DirectoryObject: models.DirectoryObject{
							ID: valToPTR("group2"),
						},
					},
				},
				"group2": {
					&models.Group{
						DirectoryObject: models.DirectoryObject{
							ID: valToPTR("group3"),
						},
					},
				},
				"group3": {
					&models.Group{
						DirectoryObject: models.DirectoryObject{
							ID: valToPTR("group4"),
						},
					},
				},
				"group4": {
					&models.Group{
						DirectoryObject: models.DirectoryObject{
							ID: valToPTR("group5"),
						},
					},
					&models.Group{
						DirectoryObject: models.DirectoryObject{
							ID: valToPTR("group6"),
						},
					},
				},
			},
			expected: map[string][]string{
				"group1": {"group1"},
				"group2": {"group2", "group1"},
				"group3": {"group3", "group2", "group1"},
				"group4": {"group4", "group3", "group2", "group1"},
				"group5": {"group5", "group4", "group3", "group2", "group1"},
				"group6": {"group6", "group4", "group3", "group2", "group1"},
			},
		},
		{
			name: "groups with cycles",
			groups: groupsByID{
				"group1": {
					DirectoryObject: models.DirectoryObject{
						ID: valToPTR("group1"),
					},
				},
				"group2": {
					DirectoryObject: models.DirectoryObject{
						ID: valToPTR("group2"),
					},
				},
				"group3": {
					DirectoryObject: models.DirectoryObject{
						ID: valToPTR("group3"),
					},
				},
			},
			groupMembers: groupMembersByGroupID{
				"group1": {
					&models.Group{
						DirectoryObject: models.DirectoryObject{
							ID: valToPTR("group2"),
						},
					},
					&models.Group{
						DirectoryObject: models.DirectoryObject{
							ID: valToPTR("group3"),
						},
					},
				},
				"group2": {
					&models.Group{
						DirectoryObject: models.DirectoryObject{
							ID: valToPTR("group1"),
						},
					},
					&models.Group{
						DirectoryObject: models.DirectoryObject{
							ID: valToPTR("group3"),
						},
					},
				},
			},
			expected: map[string][]string{
				"group1": {"group1", "group2"},
				"group2": {"group2", "group1"},
				"group3": {"group3", "group2", "group1"},
			},
		},
		{
			// `unwindGroupMembership` only calls IsOffice365Group() which checks for "Unified" type.
			name: "Office 365 groups are filtered.",
			groups: groupsByID{
				"group1": {
					DirectoryObject: models.DirectoryObject{
						ID: valToPTR("group1"),
					},
				},
				"group2": {
					DirectoryObject: models.DirectoryObject{
						ID: valToPTR("group2"),
					},
					GroupTypes: []string{"Unified"}, // o365 group.
				},
				"group3": {
					DirectoryObject: models.DirectoryObject{
						ID: valToPTR("group3"),
					},
					GroupTypes: []string{"Unified"}, // o365 group.
				},
				"group4": {
					DirectoryObject: models.DirectoryObject{
						ID: valToPTR("group4"),
					},
				},
				"group5": {
					DirectoryObject: models.DirectoryObject{
						ID: valToPTR("group5"),
					},
				},
			},
			groupMembers: groupMembersByGroupID{
				"group1": {
					&models.Group{
						DirectoryObject: models.DirectoryObject{
							ID: valToPTR("group2"),
						},
					},
					&models.Group{
						DirectoryObject: models.DirectoryObject{
							ID: valToPTR("group3"),
						},
					},
					&models.Group{
						DirectoryObject: models.DirectoryObject{
							ID: valToPTR("group4"),
						},
					},
				},
				"group2": {
					&models.Group{
						DirectoryObject: models.DirectoryObject{
							ID: valToPTR("group4"),
						},
					},
				},
				"group4": {
					&models.Group{
						DirectoryObject: models.DirectoryObject{
							ID: valToPTR("group5"),
						},
					},
				},
			},
			expected: map[string][]string{
				"group1": {"group1"},
				// group2 and group3 are filtered out.
				"group4": {"group4", "group1"},
				"group5": {"group5", "group4", "group1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := unwindGroupMembership(entraGroups{
				groupsMap:       tt.groups,
				groupMembersMap: tt.groupMembers,
			})

			sort := func(m map[string][]string) {
				for _, v := range m {
					sort.Strings(v)
				}
			}
			sort(tt.expected)
			sort(result)
			require.Equal(t, tt.expected, result)
		})
	}
}

func valToPTR[T any](v T) *T {
	return &v
}

// Test_accessListName tests the accessListName function
// to ensure that it generates a consistent name for an access list
// based on the tenant ID and group ID.
func Test_accessListName(t *testing.T) {
	type args struct {
		tenantID string
		groupID  string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "tenant and group id",
			args: args{
				tenantID: "df8ac2aa-2e0b-5fcf-b2cb-f5e210c994a3",
				groupID:  "a76a00bd-bc8e-51f5-afbd-81cf8ee5632f",
			},
			want: "9de9d997-5878-5ceb-8fab-352d7dc3c77c",
		},
		{
			name: "empty", // not possible in production
			args: args{
				tenantID: "",
				groupID:  "",
			},
			want: "cc71007b-7c2c-5bca-8466-a062e67c64f7",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := genAccessListName(tt.args.tenantID, tt.args.groupID).String()
			require.Equal(t, tt.want, got)
		})
	}
}

// createAccessListWithMembers creates a test accessListWithMembers with the given number of members
func createAccessListWithMembers(name string, numMembers int) *accessListWithMembers {
	al, err := accesslist.NewAccessList(
		header.Metadata{
			Name: name,
		},
		accesslist.Spec{
			Title: fmt.Sprintf("Access List %s", name),
			Owners: []accesslist.Owner{
				{
					Name: "owner-user",
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}

	members := make([]*accesslist.AccessListMember, numMembers)
	for i := 0; i < numMembers; i++ {
		member, err := accesslist.NewAccessListMember(
			header.Metadata{
				Name: fmt.Sprintf("user-%d", i),
			},
			accesslist.AccessListMemberSpec{
				AccessList:     name,
				Name:           fmt.Sprintf("user-%d", i),
				Joined:         time.Now().UTC(),
				AddedBy:        teleport.UserSystem,
				MembershipKind: accesslistv1.MembershipKind_MEMBERSHIP_KIND_USER.String(),
			},
		)
		if err != nil {
			panic(err)
		}
		members[i] = member
	}

	return &accessListWithMembers{
		AccessList: al,
		Members:    members,
	}
}

func BenchmarkAccessListWithMembersIsEqual(b *testing.B) {
	const (
		numAccessLists    = 50000
		avgMembersPerList = 100
	)

	accessLists := make([]*accessListWithMembers, numAccessLists)
	for i := 0; i < numAccessLists; i++ {
		name := uuid.New().String()
		accessLists[i] = createAccessListWithMembers(name, avgMembersPerList)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < numAccessLists; i++ {
		_ = accessLists[i].isEqual(accessLists[i])
	}
}

func TestDeleteNestedAccessLists(t *testing.T) {
	ctx := t.Context()
	env := NewEnv(t, newFakeGraphClient(), nil)

	// Create base access lists with allowed max depth.
	acls := make([]*accesslist.AccessList, accesslist.MaxAllowedDepth+1)
	for i := range acls {
		al := createAccessListWithMembers(fmt.Sprintf("acl-%d", i), 0).AccessList
		_, err := env.aclSvc.UpsertAccessList(ctx, al)
		require.NoError(t, err)
		acls[i] = al
	}
	// Nested relationship:
	//  acl-0
	//   -> acl-1
	//       -> acl-2
	//          ...
	//            -> acl-MaxAllowedDepth
	for i := 0; i < accesslist.MaxAllowedDepth; i++ {
		parent := acls[i]
		child := acls[i+1]

		member, err := accesslist.NewAccessListMember(
			header.Metadata{
				Name: child.GetName(),
			},
			accesslist.AccessListMemberSpec{
				AccessList:     parent.GetName(),
				Name:           child.GetName(),
				Joined:         time.Now().UTC(),
				AddedBy:        teleport.UserSystem,
				MembershipKind: accesslistv1.MembershipKind_MEMBERSHIP_KIND_LIST.String(),
			},
		)
		require.NoError(t, err)

		_, err = env.aclSvc.UpsertAccessListMember(ctx, member)
		require.NoError(t, err)
	}
	requireAccessListCount(t, env.aclSvc, len(acls))

	toDelete := make([]string, 0, len(acls))
	for _, al := range acls {
		toDelete = append(toDelete, al.GetName())
	}

	// Manually check first to prove nested access list deletion is blocked.
	err := env.aclSvc.DeleteAccessList(ctx, acls[accesslist.MaxAllowedDepth].GetName()) // acl guaranteed to be nested member
	require.ErrorIs(t, err, accesslists.ErrDeniedAccessListDeletion)

	err = deleteNestedAccessLists(ctx, env.cfg.AccessPoint, toDelete)
	require.NoError(t, err)
	requireAccessListCount(t, env.aclSvc, 0)
}

// Prove that deletion does not run forever on failed attempts.
func TestDeleteNestedAccessLists_ReturnsAfterMaxDepthRetries(t *testing.T) {
	fakeAccessPoint := &denyAclDeletion{}

	err := deleteNestedAccessLists(t.Context(), fakeAccessPoint, []string{"fake-acl-to-delete"})
	require.ErrorIs(t, err, accesslists.ErrDeniedAccessListDeletion)
	require.Equal(t, accesslist.MaxAllowedDepth+1, fakeAccessPoint.calls)
}

type denyAclDeletion struct {
	accessPoint
	calls int
}

func (d *denyAclDeletion) DeleteAccessList(ctx context.Context, name string) error {
	d.calls++
	return accesslists.ErrDeniedAccessListDeletion
}

func TestFilterOutCyclicMemberships(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	ignore := []cmp.Option{
		cmpopts.IgnoreFields(accesslist.Status{}, "OwnerOf", "MemberOf"),
		cmpopts.IgnoreFields(accesslist.AccessListMemberSpec{}, "Joined"),
	}

	tests := []struct {
		name             string
		teleportAclMap   map[string]*accessListWithMembers
		entraAclMap      map[string]*accessListWithMembers
		want             map[string]*accessListWithMembers
		errAssertionFunc require.ErrorAssertionFunc
	}{
		{
			name: "self cycle",
			entraAclMap: map[string]*accessListWithMembers{
				"listA": newAccessListWithMembers("listA", []string{"listA"}), // cycle
			},
			want: map[string]*accessListWithMembers{
				"listA": newAccessListWithMembers("listA", []string{}), // removed listA member
			},
			errAssertionFunc: func(t require.TestingT, err error, _ ...any) {
				require.ErrorIs(t, err, accesslists.ErrCyclicMembership)
			},
		},
		{
			name: "removes member that introduces cycle",
			entraAclMap: map[string]*accessListWithMembers{
				"listA": newAccessListWithMembers("listA", []string{"listB"}),
				"listB": newAccessListWithMembers("listB", []string{"listC"}),
				"listC": newAccessListWithMembers("listC", []string{"listA", "listD"}), // cycle
				"listD": newAccessListWithMembers("listD", []string{}),
			},
			want: map[string]*accessListWithMembers{
				"listA": newAccessListWithMembers("listA", []string{"listB"}),
				"listB": newAccessListWithMembers("listB", []string{"listC"}),
				"listC": newAccessListWithMembers("listC", []string{"listD"}), // removed listA
				"listD": newAccessListWithMembers("listD", []string{}),
			},
			errAssertionFunc: func(t require.TestingT, err error, _ ...any) {
				require.ErrorIs(t, err, accesslists.ErrCyclicMembership)
				require.ErrorContains(t, err, "listA")
			},
		},
		{
			name: "long chain of 5",
			entraAclMap: map[string]*accessListWithMembers{
				"listA": newAccessListWithMembers("listA", []string{"listB"}),
				"listB": newAccessListWithMembers("listB", []string{"listC"}),
				"listC": newAccessListWithMembers("listC", []string{"listD"}),
				"listD": newAccessListWithMembers("listD", []string{"listE"}),
				"listE": newAccessListWithMembers("listE", []string{"listA"}), // cycle
			},
			want: map[string]*accessListWithMembers{
				"listA": newAccessListWithMembers("listA", []string{"listB"}),
				"listB": newAccessListWithMembers("listB", []string{"listC"}),
				"listC": newAccessListWithMembers("listC", []string{"listD"}),
				"listD": newAccessListWithMembers("listD", []string{"listE"}),
				"listE": newAccessListWithMembers("listE", []string{}), // removed listA
			},
			errAssertionFunc: func(t require.TestingT, err error, _ ...any) {
				require.ErrorIs(t, err, accesslists.ErrCyclicMembership)
				require.ErrorContains(t, err, "listA")
			},
		},
		{
			name: "removes all members that introduces cycle",
			entraAclMap: map[string]*accessListWithMembers{
				"listA": newAccessListWithMembers("listA", []string{"listB"}),
				"listB": newAccessListWithMembers("listB", []string{"listA"}), // cycle
				"listC": newAccessListWithMembers("listC", []string{"listD"}),
				"listD": newAccessListWithMembers("listD", []string{"listC"}), // cycle
			},
			want: map[string]*accessListWithMembers{
				"listA": newAccessListWithMembers("listA", []string{"listB"}),
				"listB": newAccessListWithMembers("listB", []string{}), // removed cycle
				"listC": newAccessListWithMembers("listC", []string{"listD"}),
				"listD": newAccessListWithMembers("listD", []string{}), // removed cycle
			},
			errAssertionFunc: func(t require.TestingT, err error, _ ...any) {
				require.ErrorIs(t, err, accesslists.ErrCyclicMembership)
				require.ErrorContains(t, err, "listC")
				require.ErrorContains(t, err, "listA")
			},
		},
		{
			name: "acl order agnostic",
			entraAclMap: map[string]*accessListWithMembers{
				"listB": newAccessListWithMembers("listB", []string{"listA"}),
				"listA": newAccessListWithMembers("listA", []string{"listB"}), // cycle
			},
			want: map[string]*accessListWithMembers{
				"listA": newAccessListWithMembers("listA", []string{"listB"}),
				"listB": newAccessListWithMembers("listB", []string{}), // removed cycle
			},
			errAssertionFunc: func(t require.TestingT, err error, _ ...any) {
				require.ErrorIs(t, err, accesslists.ErrCyclicMembership)
				require.ErrorContains(t, err, "listB")
			},
		},
		{
			name: "member order agnostic",
			entraAclMap: map[string]*accessListWithMembers{
				"listA": newAccessListWithMembers("listA", []string{"listC", "listB"}), // cycle
				"listB": newAccessListWithMembers("listB", []string{"listA"}),
				"listC": newAccessListWithMembers("listC", []string{}),
			},
			want: map[string]*accessListWithMembers{
				"listA": newAccessListWithMembers("listA", []string{"listB", "listC"}),
				"listB": newAccessListWithMembers("listB", []string{}),
				"listC": newAccessListWithMembers("listC", []string{}), // removed listA
			},
			errAssertionFunc: func(t require.TestingT, err error, _ ...any) {
				require.ErrorIs(t, err, accesslists.ErrCyclicMembership)
				require.ErrorContains(t, err, "listA")
			},
		},
		{
			name: "existing membership preserved",
			teleportAclMap: map[string]*accessListWithMembers{
				"listA": newAccessListWithMembers("listA", []string{}),
				"listB": newAccessListWithMembers("listB", []string{"listA", "listC"}),
				"listC": newAccessListWithMembers("listC", []string{}),
			},
			entraAclMap: map[string]*accessListWithMembers{
				"listA": newAccessListWithMembers("listA", []string{"listB"}), // introduces cycle
				"listB": newAccessListWithMembers("listB", []string{"listA", "listC"}),
				"listC": newAccessListWithMembers("listC", []string{}),
			},
			want: map[string]*accessListWithMembers{
				"listA": newAccessListWithMembers("listA", []string{}),                 // removed listB
				"listB": newAccessListWithMembers("listB", []string{"listA", "listC"}), // existing edge preserved
				"listC": newAccessListWithMembers("listC", []string{}),
			},
			errAssertionFunc: func(t require.TestingT, err error, _ ...any) {
				require.ErrorIs(t, err, accesslists.ErrCyclicMembership)
				require.ErrorContains(t, err, "listB")
			},
		},
		{
			name: "new edge wins if older edge is simultaneously removed",
			teleportAclMap: map[string]*accessListWithMembers{
				"listA": newAccessListWithMembers("listA", []string{"listB"}),
				"listB": newAccessListWithMembers("listB", []string{}),
			},
			entraAclMap: map[string]*accessListWithMembers{
				"listA": newAccessListWithMembers("listA", []string{}),
				"listB": newAccessListWithMembers("listB", []string{"listA"}),
			},
			want: map[string]*accessListWithMembers{
				"listA": newAccessListWithMembers("listA", []string{}),
				"listB": newAccessListWithMembers("listB", []string{"listA"}),
			},
			errAssertionFunc: require.NoError,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := filterOutCyclicMemberships(ctx, tc.entraAclMap, tc.teleportAclMap)
			tc.errAssertionFunc(t, err)

			require.Empty(t, cmp.Diff(tc.want, tc.entraAclMap, ignore...))
		})
	}
}

func newAccessListWithMembers(name string, memberNames []string) *accessListWithMembers {
	al, err := accesslist.NewAccessList(
		header.Metadata{
			Name: name,
		},
		accesslist.Spec{
			Title: fmt.Sprintf("Access List %s", name),
			Owners: []accesslist.Owner{
				{
					Name: "owner-user",
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}
	al.SetOrigin(types.OriginEntraID)

	members := make([]*accesslist.AccessListMember, len(memberNames))
	for i, memberName := range memberNames {
		member, err := accesslist.NewAccessListMember(
			header.Metadata{
				Name: memberName,
			},
			accesslist.AccessListMemberSpec{
				AccessList:     name,
				Name:           memberName,
				Joined:         time.Now().UTC(),
				AddedBy:        teleport.UserSystem,
				MembershipKind: accesslistv1.MembershipKind_MEMBERSHIP_KIND_LIST.String(),
			},
		)
		if err != nil {
			panic(err)
		}
		members[i] = member
	}

	return &accessListWithMembers{
		AccessList: al,
		Members:    members,
	}
}

// New accessListName prevents name collision. But before this patch goes out,
// it is possible to create group display with the payload "../victimTenantID/victimGroupID",
// trying to pre seed acl name that may collide even with the new name.
// The test below proves that such collision cannot happen because of updated
// UUID namespace.
func TestAccessListNamespace(t *testing.T) {
	const (
		tenantID        = "1684dd44-e722-4103-9d58-d50cd82a3b05"
		victimGroupID   = "a54a910f-0009-41f9-a339-a983d27b62c9"
		attackerGroupID = "0017cc3f-8f7e-4aa8-9d65-bc89ebd7b2bb"
	)

	attackerDisplayName := "../" + tenantID + "/" + victimGroupID

	require.NotEqual(
		t,
		deprecatedAccessListName(attackerDisplayName, attackerGroupID),
		genAccessListName(tenantID, victimGroupID),
	)
}

func TestDetectLegacyCollisions(t *testing.T) {
	const (
		victimID      = "30e89208-e4db-421d-b275-a315b021dae3"
		victimDisplay = "groupA"

		// Two attacker groups targeting one victim.
		attackerID       = "0bc5cbc0-2cbf-4e30-bacb-f275c766d83c"
		attackerDisplay  = "../" + victimID + "/" + victimDisplay
		attacker2ID      = "94a64392-5cb0-4fa0-89b8-c6e2ade2d1f9"
		attacker2Display = "../" + victimID + "/" + victimDisplay

		otherID      = "5810cf99-e247-4224-9513-82e2307acc82"
		otherDisplay = "other-group"
	)

	groups := groupsByID{
		entraUniqueID(victimID): {
			DirectoryObject: models.DirectoryObject{
				ID:          to.Ptr(victimID),
				DisplayName: to.Ptr(victimDisplay),
			},
		},
		entraUniqueID(attackerID): {
			DirectoryObject: models.DirectoryObject{
				ID:          to.Ptr(attackerID),
				DisplayName: to.Ptr(attackerDisplay),
			},
		},
		entraUniqueID(attacker2ID): {
			DirectoryObject: models.DirectoryObject{
				ID:          to.Ptr(attacker2ID),
				DisplayName: to.Ptr(attacker2Display),
			},
		},
		entraUniqueID(otherID): {
			DirectoryObject: models.DirectoryObject{
				ID:          to.Ptr(otherID),
				DisplayName: to.Ptr(otherDisplay),
			},
		},
	}

	aclName := deprecatedAccessListName(victimDisplay, victimID)
	collisions := detectLegacyCollisions(groups)

	require.Len(t, collisions, 1)
	expectedGroups := []entraUniqueID{
		entraUniqueID(victimID),
		entraUniqueID(attackerID),
		entraUniqueID(attacker2ID),
	}
	require.ElementsMatch(t, expectedGroups, collisions[aclName])
}
