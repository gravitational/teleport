/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/trace"
	"github.com/vulcand/predicate"
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
