package accesslist

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/accesslist"
)

func Test_isReviewChangesAllowed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   accesslist.ReviewChanges
		want bool
	}{
		{
			name: "empty review changes are allowed",
			in:   accesslist.ReviewChanges{},
			want: true,
		},
		{
			name: "only removed members is allowed",
			in: accesslist.ReviewChanges{
				RemovedMembers: []string{"user1", "user2"},
			},
			want: true,
		},
		{
			name: "membership requirements changed is not allowed",
			in: accesslist.ReviewChanges{
				MembershipRequirementsChanged: &accesslist.Requires{
					Roles: []string{"role1"},
				},
			},
			want: false,
		},
		{
			name: "review frequency changed is not allowed",
			in: accesslist.ReviewChanges{
				ReviewFrequencyChanged: accesslist.ThreeMonths,
			},
			want: false,
		},
		{
			name: "review day of month changed is not allowed",
			in: accesslist.ReviewChanges{
				ReviewDayOfMonthChanged: accesslist.FifteenthDayOfMonth,
			},
			want: false,
		},
		{
			name: "removed members with membership requirements changed is not allowed",
			in: accesslist.ReviewChanges{
				RemovedMembers: []string{"user1"},
				MembershipRequirementsChanged: &accesslist.Requires{
					Roles: []string{"role1"},
				},
			},
			want: false,
		},
		{
			name: "removed members with review frequency changed is not allowed",
			in: accesslist.ReviewChanges{
				RemovedMembers:         []string{"user1"},
				ReviewFrequencyChanged: accesslist.SixMonths,
			},
			want: false,
		},
		{
			name: "removed members with review day of month changed is not allowed",
			in: accesslist.ReviewChanges{
				RemovedMembers:          []string{"user1"},
				ReviewDayOfMonthChanged: accesslist.FirstDayOfMonth,
			},
			want: false,
		},
		{
			name: "all non-ignored fields changed is not allowed",
			in: accesslist.ReviewChanges{
				MembershipRequirementsChanged: &accesslist.Requires{
					Roles: []string{"role1"},
				},
				ReviewFrequencyChanged:  accesslist.ThreeMonths,
				ReviewDayOfMonthChanged: accesslist.FifteenthDayOfMonth,
			},
			want: false,
		},
		{
			name: "all fields populated is not allowed",
			in: accesslist.ReviewChanges{
				RemovedMembers: []string{"user1"},
				MembershipRequirementsChanged: &accesslist.Requires{
					Roles: []string{"role1"},
				},
				ReviewFrequencyChanged:  accesslist.ThreeMonths,
				ReviewDayOfMonthChanged: accesslist.FifteenthDayOfMonth,
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isReviewChangesAllowed(tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}
