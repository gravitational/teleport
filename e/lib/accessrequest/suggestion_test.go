package accessrequest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
)

func TestScoreRelevance(t *testing.T) {
	t.Parallel()

	makeAccessList := func(roles ...string) *accesslist.AccessList {
		return &accesslist.AccessList{
			Spec: accesslist.Spec{
				Grants: accesslist.Grants{Roles: roles},
			},
		}
	}

	type args struct {
		request types.AccessRequest
		lists   []*accesslist.AccessList
	}
	tests := []struct {
		name string
		args args
		want []*accesslist.AccessList
	}{
		{
			name: "no lists",
			args: args{
				request: &types.AccessRequestV3{},
				lists:   []*accesslist.AccessList{},
			},
			want: []*accesslist.AccessList{},
		},
		{
			name: "one role",
			args: args{
				request: &types.AccessRequestV3{
					Spec: types.AccessRequestSpecV3{Roles: []string{"role1"}},
				},
				lists: []*accesslist.AccessList{
					makeAccessList("role1"),
				},
			},
			want: []*accesslist.AccessList{
				makeAccessList("role1"),
			},
		},
		{
			name: "irrelevant roles are penalized",
			args: args{
				request: &types.AccessRequestV3{
					Spec: types.AccessRequestSpecV3{Roles: []string{"role1"}},
				},
				lists: []*accesslist.AccessList{
					makeAccessList("role1", "role2"),
					makeAccessList("role1"),
				},
			},
			want: []*accesslist.AccessList{
				makeAccessList("role1"),
				makeAccessList("role1", "role2"),
			},
		},
		{
			name: "relevant roles are prioritized",
			args: args{
				request: &types.AccessRequestV3{
					Spec: types.AccessRequestSpecV3{Roles: []string{"role1", "role3"}},
				},
				lists: []*accesslist.AccessList{
					makeAccessList("role100", "role101"),
					makeAccessList("role1", "role3"),
					makeAccessList("role1", "role2"),
				},
			},
			want: []*accesslist.AccessList{
				makeAccessList("role1", "role3"),
				makeAccessList("role1", "role2"),
				makeAccessList("role100", "role101"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScoreRelevance(tt.args.request, tt.args.lists)
			require.Equal(t, tt.want, got)
		})
	}
}
