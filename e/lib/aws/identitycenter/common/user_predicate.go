package identitycentercommon

import "github.com/gravitational/teleport/api/types"

// UserFilterFunc is a function that filters users.
type UserFilterFunc func(types.User) bool

// UserPredicateFilter returns a predicate function that filters users based on the
// provided filters.
func UserPredicateFilter(filters []*types.AWSICUserSyncFilter) func(u types.User) bool {
	return func(u types.User) bool {
		if types.IsSystemResource(u) {
			return false
		}
		if len(filters) == 0 {
			return true
		}
		for _, v := range filters {
			if types.MatchLabels(u, v.Labels) {
				return true
			}
		}
		return false
	}
}
