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

package services

import (
	"strings"

	"github.com/gravitational/trace"
	"github.com/vulcand/predicate"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
)

// impersonateContext is a default rule context used in teleport
type impersonateContext struct {
	// user is currently authenticated user
	user types.User
	// impersonateRole is a role to impersonate
	impersonateRole types.Role
	// impersonateUser is a user to impersonate
	impersonateUser types.User
}

// getIdentifier returns identifier defined in a context
func (ctx *impersonateContext) getIdentifier(fields []string) (interface{}, error) {
	switch fields[0] {
	case UserIdentifier:
		return predicate.GetFieldByTag(ctx.user, teleport.JSON, fields[1:])
	case ImpersonateUserIdentifier:
		return predicate.GetFieldByTag(ctx.impersonateUser, teleport.JSON, fields[1:])
	case ImpersonateRoleIdentifier:
		return predicate.GetFieldByTag(ctx.impersonateRole, teleport.JSON, fields[1:])
	default:
		return nil, trace.NotFound("%v is not defined", strings.Join(fields, "."))
	}
}

// matchesImpersonateWhere returns true if Where rule matches.
// Empty Where block always matches.
func matchesImpersonateWhere(cond types.ImpersonateConditions, parser predicate.Parser) (bool, error) {
	if cond.Where == "" {
		return true, nil
	}
	ifn, err := parser.Parse(cond.Where)
	if err != nil {
		return false, trace.Wrap(err)
	}
	fn, ok := ifn.(predicate.BoolPredicate)
	if !ok {
		return false, trace.BadParameter("invalid predicate type for where expression: %v", cond.Where)
	}
	return fn(), nil
}

// newImpersonateWhereParser returns standard parser for `where` section in impersonate rules
func newImpersonateWhereParser(ctx *impersonateContext) (predicate.Parser, error) {
	return predicate.NewParser(predicate.Def{
		Operators: predicate.Operators{
			AND: predicate.And,
			OR:  predicate.Or,
			NOT: predicate.Not,
		},
		Functions: map[string]interface{}{
			"equals":   predicate.Equals,
			"contains": predicate.Contains,
		},
		GetIdentifier: ctx.getIdentifier,
		GetProperty:   GetStringMapValue,
	})
}
