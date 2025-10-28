package entraid

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/msgraph"
)

func TestUnwindGroupMembership(t *testing.T) {
	tests := []struct {
		name         string
		groups       map[string]*msgraph.Group
		groupMembers map[string][]msgraph.GroupMember
		expected     map[string][]string
	}{
		{
			name:         "empty input",
			groups:       map[string]*msgraph.Group{},
			groupMembers: map[string][]msgraph.GroupMember{},
			expected:     map[string][]string{},
		},
		{
			name: "single group",
			groups: map[string]*msgraph.Group{
				"group1": {
					DirectoryObject: msgraph.DirectoryObject{
						ID: valToPTR("group1"),
					},
				},
			},
			groupMembers: map[string][]msgraph.GroupMember{},
			expected: map[string][]string{
				"group1": {"group1"},
			},
		},
		{
			name: "groups with other groups as members",
			groups: map[string]*msgraph.Group{
				"group1": {
					DirectoryObject: msgraph.DirectoryObject{
						ID: valToPTR("group1"),
					},
				},
				"group2": {
					DirectoryObject: msgraph.DirectoryObject{
						ID: valToPTR("group2"),
					},
				},
				"group3": {
					DirectoryObject: msgraph.DirectoryObject{
						ID: valToPTR("group3"),
					},
				},
			},
			groupMembers: map[string][]msgraph.GroupMember{
				"group1": {
					&msgraph.Group{
						DirectoryObject: msgraph.DirectoryObject{
							ID: valToPTR("group2"),
						},
					},
					&msgraph.Group{
						DirectoryObject: msgraph.DirectoryObject{
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
			groups: map[string]*msgraph.Group{
				"group1": {
					DirectoryObject: msgraph.DirectoryObject{
						ID: valToPTR("group1"),
					},
				},
				"group2": {
					DirectoryObject: msgraph.DirectoryObject{
						ID: valToPTR("group2"),
					},
				},
				"group3": {
					DirectoryObject: msgraph.DirectoryObject{
						ID: valToPTR("group3"),
					},
				},
				"group4": {
					DirectoryObject: msgraph.DirectoryObject{
						ID: valToPTR("group4"),
					},
				},
				"group5": {
					DirectoryObject: msgraph.DirectoryObject{
						ID: valToPTR("group5"),
					},
				},
				"group6": {
					DirectoryObject: msgraph.DirectoryObject{
						ID: valToPTR("group6"),
					},
				},
			},
			groupMembers: map[string][]msgraph.GroupMember{
				"group1": {
					&msgraph.Group{
						DirectoryObject: msgraph.DirectoryObject{
							ID: valToPTR("group2"),
						},
					},
				},
				"group2": {
					&msgraph.Group{
						DirectoryObject: msgraph.DirectoryObject{
							ID: valToPTR("group3"),
						},
					},
				},
				"group3": {
					&msgraph.Group{
						DirectoryObject: msgraph.DirectoryObject{
							ID: valToPTR("group4"),
						},
					},
				},
				"group4": {
					&msgraph.Group{
						DirectoryObject: msgraph.DirectoryObject{
							ID: valToPTR("group5"),
						},
					},
					&msgraph.Group{
						DirectoryObject: msgraph.DirectoryObject{
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
			groups: map[string]*msgraph.Group{
				"group1": {
					DirectoryObject: msgraph.DirectoryObject{
						ID: valToPTR("group1"),
					},
				},
				"group2": {
					DirectoryObject: msgraph.DirectoryObject{
						ID: valToPTR("group2"),
					},
				},
				"group3": {
					DirectoryObject: msgraph.DirectoryObject{
						ID: valToPTR("group3"),
					},
				},
			},
			groupMembers: map[string][]msgraph.GroupMember{
				"group1": {
					&msgraph.Group{
						DirectoryObject: msgraph.DirectoryObject{
							ID: valToPTR("group2"),
						},
					},
					&msgraph.Group{
						DirectoryObject: msgraph.DirectoryObject{
							ID: valToPTR("group3"),
						},
					},
				},
				"group2": {
					&msgraph.Group{
						DirectoryObject: msgraph.DirectoryObject{
							ID: valToPTR("group1"),
						},
					},
					&msgraph.Group{
						DirectoryObject: msgraph.DirectoryObject{
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := unwindGroupMembership(tt.groups, tt.groupMembers)

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
// based on the display name and ID.
func Test_accessListName(t *testing.T) {
	type args struct {
		displayName string
		id          string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "valid display name",
			args: args{
				displayName: "test",
				id:          "123",
			},
			want: "a76a00bd-bc8e-51f5-afbd-81cf8ee5632f",
		},
		{
			name: "invalid display name",
			args: args{
				displayName: "test[]^!#$",
				id:          "123",
			},
			want: "df8ac2aa-2e0b-5fcf-b2cb-f5e210c994a3",
		},
		{
			name: "empty",
			args: args{
				displayName: "",
				id:          "",
			},
			want: "1f81d2df-49d5-53d5-b54a-cb9680d84e1e",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := accessListName(tt.args.displayName, tt.args.id)
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
