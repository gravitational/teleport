/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestFields(t *testing.T) {
	t.Parallel()

	now := time.Now().Round(time.Minute)

	sliceString := []string{"test", "string", "slice"}
	sliceInterface := []any{"test", "string", "slice"}
	f := Fields{
		"one":      1,
		"name":     "vincent",
		"time":     now,
		"strings":  sliceString,
		"strings2": sliceInterface,
	}

	require.Equal(t, 1, f.GetInt("one"))
	require.Equal(t, 0, f.GetInt("two"))
	require.Equal(t, "vincent", f.GetString("name"))
	require.Empty(t, f.GetString("city"))
	require.Equal(t, now, f.GetTime("time"))
	require.Equal(t, sliceString, f.GetStrings("strings"))
	require.Equal(t, sliceString, f.GetStrings("strings2"))
	require.Nil(t, f.GetStrings("strings3"))
}

func TestToFieldsCondition(t *testing.T) {
	t.Parallel()

	// !equals(login, "root") && contains(participants, "test-user")
	expr := &types.WhereExpr{And: types.WhereExpr2{
		L: &types.WhereExpr{Not: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "login"}, R: &types.WhereExpr{Literal: "root"}}}},
		R: &types.WhereExpr{Contains: types.WhereExpr2{L: &types.WhereExpr{Field: "participants"}, R: &types.WhereExpr{Literal: "test-user"}}},
	}}

	cond, err := ToFieldsCondition(ToFieldsConditionConfig{
		Expr: expr,
	})
	require.NoError(t, err)

	require.False(t, cond(Fields{}))
	require.False(t, cond(Fields{"login": "root", "participants": []string{"test-user", "observer"}}))
	require.False(t, cond(Fields{"login": "guest", "participants": []string{"another-user"}}))
	require.True(t, cond(Fields{"login": "guest", "participants": []string{"test-user", "observer"}}))
	require.True(t, cond(Fields{"participants": []string{"test-user"}}))
}

func TestToFieldsConditionNilExpression(t *testing.T) {
	t.Parallel()

	_, err := ToFieldsCondition(ToFieldsConditionConfig{
		Expr: nil,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "expr is nil")
}

func TestToFieldsConditionLogicalOperators(t *testing.T) {
	t.Parallel()

	t.Run("AND", func(t *testing.T) {
		// name == "alice" AND role == "admin"
		expr := &types.WhereExpr{And: types.WhereExpr2{
			L: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "name"}, R: &types.WhereExpr{Literal: "alice"}}},
			R: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "role"}, R: &types.WhereExpr{Literal: "admin"}}},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		// Both conditions satisfied
		require.True(t, cond(Fields{"name": "alice", "role": "admin"}))
		// One condition satisfied
		require.False(t, cond(Fields{"name": "alice", "role": "user"}))
		require.False(t, cond(Fields{"name": "bob", "role": "admin"}))
		// No conditions satisfied
		require.False(t, cond(Fields{"name": "bob", "role": "user"}))
	})

	t.Run("OR", func(t *testing.T) {
		// name == "alice" OR role == "admin"
		expr := &types.WhereExpr{Or: types.WhereExpr2{
			L: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "name"}, R: &types.WhereExpr{Literal: "alice"}}},
			R: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "role"}, R: &types.WhereExpr{Literal: "admin"}}},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		// Both conditions satisfied
		require.True(t, cond(Fields{"name": "alice", "role": "admin"}))
		// One condition satisfied
		require.True(t, cond(Fields{"name": "alice", "role": "user"}))
		require.True(t, cond(Fields{"name": "bob", "role": "admin"}))
		// No conditions satisfied
		require.False(t, cond(Fields{"name": "bob", "role": "user"}))
	})

	t.Run("NOT", func(t *testing.T) {
		// NOT (name == "alice")
		expr := &types.WhereExpr{Not: &types.WhereExpr{
			Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "name"}, R: &types.WhereExpr{Literal: "alice"}},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.False(t, cond(Fields{"name": "alice"}))
		require.True(t, cond(Fields{"name": "bob"}))
	})

	t.Run("complex expression", func(t *testing.T) {
		// (name == "alice" OR role == "admin") AND NOT (dept == "finance")
		expr := &types.WhereExpr{And: types.WhereExpr2{
			L: &types.WhereExpr{Or: types.WhereExpr2{
				L: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "name"}, R: &types.WhereExpr{Literal: "alice"}}},
				R: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "role"}, R: &types.WhereExpr{Literal: "admin"}}},
			}},
			R: &types.WhereExpr{Not: &types.WhereExpr{
				Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "dept"}, R: &types.WhereExpr{Literal: "finance"}},
			}},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		// Satisfies both conditions
		require.True(t, cond(Fields{"name": "alice", "dept": "engineering"}))
		require.True(t, cond(Fields{"name": "bob", "role": "admin", "dept": "sales"}))

		// Fails the NOT condition
		require.False(t, cond(Fields{"name": "alice", "dept": "finance"}))
		require.False(t, cond(Fields{"role": "admin", "dept": "finance"}))

		// Fails the OR condition
		require.False(t, cond(Fields{"name": "bob", "role": "user", "dept": "engineering"}))
	})
}

func TestToFieldsConditionEqualsOperations(t *testing.T) {
	t.Parallel()

	t.Run("field-field", func(t *testing.T) {
		// name == alias
		expr := &types.WhereExpr{Equals: types.WhereExpr2{
			L: &types.WhereExpr{Field: "name"},
			R: &types.WhereExpr{Field: "alias"},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{"name": "alice", "alias": "alice"}))
		require.False(t, cond(Fields{"name": "alice", "alias": "bob"}))
		require.False(t, cond(Fields{"name": "alice"})) // missing field
	})

	t.Run("field-literal", func(t *testing.T) {
		// name == "alice"
		expr := &types.WhereExpr{Equals: types.WhereExpr2{
			L: &types.WhereExpr{Field: "name"},
			R: &types.WhereExpr{Literal: "alice"},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{"name": "alice"}))
		require.False(t, cond(Fields{"name": "bob"}))
		require.False(t, cond(Fields{})) // missing field
	})

	t.Run("literal-field", func(t *testing.T) {
		// "alice" == name
		expr := &types.WhereExpr{Equals: types.WhereExpr2{
			L: &types.WhereExpr{Literal: "alice"},
			R: &types.WhereExpr{Field: "name"},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{"name": "alice"}))
		require.False(t, cond(Fields{"name": "bob"}))
		require.False(t, cond(Fields{})) // missing field
	})
}

func TestToFieldsConditionContainsOperations(t *testing.T) {
	t.Parallel()

	t.Run("field-field", func(t *testing.T) {
		// contains(teams, leader)
		expr := &types.WhereExpr{Contains: types.WhereExpr2{
			L: &types.WhereExpr{Field: "teams"},
			R: &types.WhereExpr{Field: "leader"},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{
			"teams":  []string{"engineering", "frontend", "backend"},
			"leader": "frontend",
		}))
		require.False(t, cond(Fields{
			"teams":  []string{"engineering", "frontend", "backend"},
			"leader": "design",
		}))
		require.False(t, cond(Fields{"teams": []string{"engineering"}})) // missing field
	})

	t.Run("field-literal", func(t *testing.T) {
		// contains(teams, "frontend")
		expr := &types.WhereExpr{Contains: types.WhereExpr2{
			L: &types.WhereExpr{Field: "teams"},
			R: &types.WhereExpr{Literal: "frontend"},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{"teams": []string{"engineering", "frontend", "backend"}}))
		require.False(t, cond(Fields{"teams": []string{"engineering", "backend"}}))
		require.False(t, cond(Fields{})) // missing field
	})

	t.Run("literal-field", func(t *testing.T) {
		// contains(["admin", "superuser"], role)
		expr := &types.WhereExpr{Contains: types.WhereExpr2{
			L: &types.WhereExpr{Literal: []string{"admin", "superuser"}},
			R: &types.WhereExpr{Field: "role"},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{"role": "admin"}))
		require.True(t, cond(Fields{"role": "superuser"}))
		require.False(t, cond(Fields{"role": "user"}))
		require.False(t, cond(Fields{})) // missing field
	})

	t.Run("interface slice", func(t *testing.T) {
		// contains(teams, "frontend")
		expr := &types.WhereExpr{Contains: types.WhereExpr2{
			L: &types.WhereExpr{Field: "teams"},
			R: &types.WhereExpr{Literal: "frontend"},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{"teams": []any{"engineering", "frontend", "backend"}}))
		require.False(t, cond(Fields{"teams": []any{"engineering", "backend"}}))
	})
}

func TestToFieldsConditionContainsAllOperations(t *testing.T) {
	t.Parallel()

	t.Run("field-field", func(t *testing.T) {
		// contains_all(user_teams, required_teams)
		expr := &types.WhereExpr{ContainsAll: types.WhereExpr2{
			L: &types.WhereExpr{Field: "user_teams"},
			R: &types.WhereExpr{Field: "required_teams"},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{
			"user_teams":     []string{"engineering", "frontend", "backend"},
			"required_teams": []string{"engineering", "frontend"},
		}))
		require.False(t, cond(Fields{
			"user_teams":     []string{"engineering", "frontend"},
			"required_teams": []string{"engineering", "frontend", "backend"},
		}))
		require.False(t, cond(Fields{
			"user_teams":     []string{"engineering", "frontend"},
			"required_teams": []string{"engineering", "backend"},
		}))
	})

	t.Run("field-literal", func(t *testing.T) {
		// contains_all(user_teams, ["engineering", "frontend"])
		expr := &types.WhereExpr{ContainsAll: types.WhereExpr2{
			L: &types.WhereExpr{Field: "user_teams"},
			R: &types.WhereExpr{Literal: []string{"engineering", "frontend"}},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{"user_teams": []string{"engineering", "frontend", "backend"}}))
		require.True(t, cond(Fields{"user_teams": []string{"engineering", "frontend"}}))
		require.False(t, cond(Fields{"user_teams": []string{"engineering", "backend"}}))
		require.False(t, cond(Fields{})) // missing field
	})

	t.Run("literal-field", func(t *testing.T) {
		// contains_all(["admin", "superuser", "manager"], user_roles)
		expr := &types.WhereExpr{ContainsAll: types.WhereExpr2{
			L: &types.WhereExpr{Literal: []string{"admin", "superuser", "manager"}},
			R: &types.WhereExpr{Field: "user_roles"},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{"user_roles": []string{"admin", "superuser"}}))
		require.True(t, cond(Fields{"user_roles": []string{"admin"}}))
		require.False(t, cond(Fields{"user_roles": []string{"admin", "superuser", "manager", "extra"}}))
		require.False(t, cond(Fields{})) // missing field
	})
}

func TestToFieldsConditionContainsAnyOperations(t *testing.T) {
	t.Parallel()

	t.Run("field-field", func(t *testing.T) {
		// contains_any(user_roles, allowed_roles)
		expr := &types.WhereExpr{ContainsAny: types.WhereExpr2{
			L: &types.WhereExpr{Field: "user_roles"},
			R: &types.WhereExpr{Field: "allowed_roles"},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{
			"user_roles":    []string{"admin", "manager"},
			"allowed_roles": []string{"admin", "superuser", "operator"},
		}))
		require.False(t, cond(Fields{
			"user_roles":    []string{"manager", "developer"},
			"allowed_roles": []string{"admin", "superuser", "operator"},
		}))
	})

	t.Run("field-literal", func(t *testing.T) {
		// contains_any(user_roles, ["admin", "superuser"])
		expr := &types.WhereExpr{ContainsAny: types.WhereExpr2{
			L: &types.WhereExpr{Field: "user_roles"},
			R: &types.WhereExpr{Literal: []string{"admin", "superuser"}},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{"user_roles": []string{"admin", "manager"}}))
		require.True(t, cond(Fields{"user_roles": []string{"superuser"}}))
		require.False(t, cond(Fields{"user_roles": []string{"manager", "operator"}}))
		require.False(t, cond(Fields{})) // missing field
	})

	t.Run("literal-field", func(t *testing.T) {
		// contains_any(["admin", "superuser"], user_role)
		expr := &types.WhereExpr{ContainsAny: types.WhereExpr2{
			L: &types.WhereExpr{Literal: []string{"admin", "superuser"}},
			R: &types.WhereExpr{Field: "user_role"},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{"user_role": []string{"admin"}}))
		require.True(t, cond(Fields{"user_role": []string{"superuser"}}))
		require.False(t, cond(Fields{"user_role": []string{"manager"}}))
		require.False(t, cond(Fields{})) // missing field
	})
}

func TestToFieldsConditionMapReferences(t *testing.T) {
	t.Parallel()

	t.Run("map equals left", func(t *testing.T) {
		// labels["env"] == "production"
		expr := &types.WhereExpr{Equals: types.WhereExpr2{
			L: &types.WhereExpr{MapRef: &types.WhereExpr2{
				L: &types.WhereExpr{Field: "labels"},
				R: &types.WhereExpr{Literal: "env"},
			}},
			R: &types.WhereExpr{Literal: "production"},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{"labels": map[string]any{"env": "production", "app": "web"}}))
		require.False(t, cond(Fields{"labels": map[string]any{"env": "staging", "app": "web"}}))
		require.False(t, cond(Fields{"labels": map[string]any{"app": "web"}})) // key not present
		require.False(t, cond(Fields{}))                                       // map not present
	})

	t.Run("map equals right", func(t *testing.T) {
		// "production" == labels["env"]
		expr := &types.WhereExpr{Equals: types.WhereExpr2{
			L: &types.WhereExpr{Literal: "production"},
			R: &types.WhereExpr{MapRef: &types.WhereExpr2{
				L: &types.WhereExpr{Field: "labels"},
				R: &types.WhereExpr{Literal: "env"},
			}},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{"labels": map[string]any{"env": "production", "app": "web"}}))
		require.False(t, cond(Fields{"labels": map[string]any{"env": "staging", "app": "web"}}))
		require.False(t, cond(Fields{"labels": map[string]any{"app": "web"}})) // key not present
		require.False(t, cond(Fields{}))                                       // map not present
	})

	t.Run("map contains left", func(t *testing.T) {
		// contains(labels["roles"], "admin")
		expr := &types.WhereExpr{Contains: types.WhereExpr2{
			L: &types.WhereExpr{MapRef: &types.WhereExpr2{
				L: &types.WhereExpr{Field: "labels"},
				R: &types.WhereExpr{Literal: "roles"},
			}},
			R: &types.WhereExpr{Literal: "admin"},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{
			"labels": map[string]any{"roles": []string{"admin", "user"}, "app": "web"},
		}))
		require.False(t, cond(Fields{
			"labels": map[string]any{"roles": []string{"user", "guest"}, "app": "web"},
		}))
		require.False(t, cond(Fields{"labels": map[string]any{"app": "web"}})) // key not present
		require.False(t, cond(Fields{}))                                       // map not present
	})

	t.Run("map contains right", func(t *testing.T) {
		// contains(["admin"],labels["role"])
		expr := &types.WhereExpr{Contains: types.WhereExpr2{
			L: &types.WhereExpr{Literal: []string{"admin"}},
			R: &types.WhereExpr{MapRef: &types.WhereExpr2{
				L: &types.WhereExpr{Field: "labels"},
				R: &types.WhereExpr{Literal: "role"},
			}},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{
			"labels": map[string]any{"role": "admin", "app": "web"},
		}))
		require.False(t, cond(Fields{
			"labels": map[string]any{"role": "user", "app": "web"},
		}))
		require.False(t, cond(Fields{"labels": map[string]any{"app": "web"}})) // key not present
		require.False(t, cond(Fields{}))                                       // map not present
	})

	t.Run("map contains_al left ", func(t *testing.T) {
		// contains_all(labels["roles"], ["admin", "user"])
		expr := &types.WhereExpr{ContainsAll: types.WhereExpr2{
			L: &types.WhereExpr{MapRef: &types.WhereExpr2{
				L: &types.WhereExpr{Field: "labels"},
				R: &types.WhereExpr{Literal: "roles"},
			}},
			R: &types.WhereExpr{Literal: []string{"admin", "user"}},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{
			"labels": map[string]any{"roles": []string{"admin", "user", "guest"}, "app": "web"},
		}))
		require.False(t, cond(Fields{
			"labels": map[string]any{"roles": []string{"admin", "guest"}, "app": "web"},
		}))
	})

	t.Run("map contains_al right", func(t *testing.T) {
		// contains_all(["admin", "user"],labels["roles"])
		expr := &types.WhereExpr{ContainsAll: types.WhereExpr2{
			L: &types.WhereExpr{Literal: []string{"admin", "user"}},
			R: &types.WhereExpr{MapRef: &types.WhereExpr2{
				L: &types.WhereExpr{Field: "labels"},
				R: &types.WhereExpr{Literal: "roles"},
			}},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{
			"labels": map[string]any{"roles": []string{"admin", "user"}, "app": "web"},
		}))
		require.False(t, cond(Fields{
			"labels": map[string]any{"roles": []string{"admin", "guest"}, "app": "web"},
		}))
	})

	t.Run("map contains_any left", func(t *testing.T) {
		// contains_any(labels["roles"], ["admin", "superuser"])
		expr := &types.WhereExpr{ContainsAny: types.WhereExpr2{
			L: &types.WhereExpr{MapRef: &types.WhereExpr2{
				L: &types.WhereExpr{Field: "labels"},
				R: &types.WhereExpr{Literal: "roles"},
			}},
			R: &types.WhereExpr{Literal: []string{"admin", "superuser"}},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{
			"labels": map[string]any{"roles": []string{"admin", "user"}, "app": "web"},
		}))
		require.True(t, cond(Fields{
			"labels": map[string]any{"roles": []string{"superuser", "guest"}, "app": "web"},
		}))
		require.False(t, cond(Fields{
			"labels": map[string]any{"roles": []string{"user", "guest"}, "app": "web"},
		}))
	})

	t.Run("map contains_any right", func(t *testing.T) {
		// contains_any(["admin", "superuser"],labels["roles"])
		expr := &types.WhereExpr{ContainsAny: types.WhereExpr2{
			L: &types.WhereExpr{Literal: []string{"admin", "superuser"}},
			R: &types.WhereExpr{MapRef: &types.WhereExpr2{
				L: &types.WhereExpr{Field: "labels"},
				R: &types.WhereExpr{Literal: "roles"},
			}},
		}}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{Expr: expr})
		require.NoError(t, err)

		require.True(t, cond(Fields{
			"labels": map[string]any{"roles": []string{"admin", "user"}, "app": "web"},
		}))
		require.True(t, cond(Fields{
			"labels": map[string]any{"roles": []string{"superuser", "guest"}, "app": "web"},
		}))
		require.False(t, cond(Fields{
			"labels": map[string]any{"roles": []string{"user", "guest"}, "app": "web"},
		}))
	})
}

func TestToFieldsConditionCanView(t *testing.T) {
	t.Parallel()

	// can_view()
	expr := &types.WhereExpr{CanView: &types.WhereNoExpr{}}

	t.Run("with canView function", func(t *testing.T) {
		// Provide a canView function that only allows access to resources with allow=true
		canView := func(f Fields) bool {
			allow, ok := f["allow"]
			if !ok {
				return false
			}
			return allow.(bool)
		}

		cond, err := ToFieldsCondition(ToFieldsConditionConfig{
			Expr:    expr,
			CanView: canView,
		})
		require.NoError(t, err)

		require.True(t, cond(Fields{"allow": true, "name": "resource1"}))
		require.False(t, cond(Fields{"allow": false, "name": "resource2"}))
		require.False(t, cond(Fields{"name": "resource3"}))
	})

	t.Run("without canView function", func(t *testing.T) {
		_, err := ToFieldsCondition(ToFieldsConditionConfig{
			Expr: expr,
			// No canView function provided
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "canView expression provided but no canView function specified")
	})
}

func TestToFieldsConditionInvalidExpression(t *testing.T) {
	t.Parallel()

	// Create an empty expression with no operators
	expr := &types.WhereExpr{}

	_, err := ToFieldsCondition(ToFieldsConditionConfig{
		Expr: expr,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to convert expression")
}
