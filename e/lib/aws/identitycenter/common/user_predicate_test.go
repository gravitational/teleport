package identitycentercommon

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestUserPredicateFilter(t *testing.T) {
	mkUser := func(labels map[string]string) types.User {
		return &types.UserV2{
			Metadata: types.Metadata{
				Labels: labels,
			},
		}
	}

	tests := []struct {
		name     string
		filters  []*types.AWSICUserSyncFilter
		user     types.User
		expected bool
	}{
		{
			name:    "system resource user should be filtered out",
			filters: nil,
			user: mkUser(
				map[string]string{
					types.TeleportInternalResourceType: types.SystemResource,
				},
			),
			expected: false,
		},
		{
			name:     "user with no filters should be included",
			filters:  nil,
			user:     mkUser(nil),
			expected: true,
		},
		{
			name: "user with no matching filter should be excluded",
			filters: []*types.AWSICUserSyncFilter{
				{Labels: map[string]string{types.OriginLabel: types.OriginOkta}},
			},
			user:     mkUser(nil),
			expected: false,
		},
		{
			name: "user with matching filter should be included",
			filters: []*types.AWSICUserSyncFilter{
				{Labels: map[string]string{types.OriginLabel: types.OriginOkta}},
			},
			user: mkUser(map[string]string{
				types.OriginLabel: types.OriginOkta,
			}),
			expected: true,
		},
		{
			name: "system user with  matching filter should be excluded",
			filters: []*types.AWSICUserSyncFilter{
				{Labels: map[string]string{types.OriginLabel: types.OriginOkta}},
			},
			user: mkUser(map[string]string{
				types.TeleportInternalResourceType: types.SystemResource,
				types.OriginLabel:                  types.OriginOkta,
			}),
			expected: false,
		},
		{
			name: "user with matching second filter should be included",
			filters: []*types.AWSICUserSyncFilter{
				{Labels: map[string]string{"foo": "bar"}},
				{Labels: map[string]string{types.OriginLabel: types.OriginOkta}},
			},
			user: mkUser(map[string]string{
				types.OriginLabel: types.OriginOkta,
			}),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			predicate := UserPredicateFilter(tt.filters)
			require.Equal(t, tt.expected, predicate(tt.user))
		})
	}
}
