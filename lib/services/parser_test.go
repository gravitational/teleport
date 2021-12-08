/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either exptypes.WhereExprs or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestParserForIdentifierSubcondition(t *testing.T) {
	t.Parallel()
	user, err := types.NewUser("test-user")
	require.NoError(t, err)
	testCase := func(cond string, expected types.WhereExpr) func(*testing.T) {
		return func(t *testing.T) {
			parser, err := newParserForIdentifierSubcondition(&Context{User: user}, SessionIdentifier)
			require.NoError(t, err)
			out, err := parser.Parse(cond)
			require.NoError(t, err)
			expr := out.(types.WhereExpr)
			require.Empty(t, cmp.Diff(expected, expr))
		}
	}

	t.Run("simple condition, with identifier #1",
		testCase(`contains(session.participants, "test")`,
			types.WhereExpr{Contains: types.WhereExpr2{
				L: &types.WhereExpr{Field: "participants"},
				R: &types.WhereExpr{Literal: "test"},
			}}))
	t.Run("simple condition, with identifier #2",
		testCase(`contains(session.participants, session.login)`,
			types.WhereExpr{Contains: types.WhereExpr2{
				L: &types.WhereExpr{Field: "participants"},
				R: &types.WhereExpr{Field: "login"},
			}}))
	t.Run("simple condition, without identifier (true)",
		testCase(`equals(user.metadata.name, "test-user")`, types.WhereExpr{Literal: true}))
	t.Run("simple condition, without identifier (false)",
		testCase(`equals(user.metadata.name, "test-user2")`, types.WhereExpr{Literal: false}))
	t.Run("simple condition, without identifier (negated false)",
		testCase(`!equals(user.metadata.name, "test-user2")`, types.WhereExpr{Literal: true}))

	t.Run("and-condition, with identifier",
		testCase(`contains(session.participants, "test") && equals(session.login, "root")`,
			types.WhereExpr{And: types.WhereExpr2{
				L: &types.WhereExpr{Contains: types.WhereExpr2{
					L: &types.WhereExpr{Field: "participants"},
					R: &types.WhereExpr{Literal: "test"},
				}},
				R: &types.WhereExpr{Equals: types.WhereExpr2{
					L: &types.WhereExpr{Field: "login"},
					R: &types.WhereExpr{Literal: "root"},
				}}}}))
	t.Run("and-condition, with identifier (negated)",
		testCase(`!(contains(session.participants, "test") && equals(session.login, "root"))`,
			types.WhereExpr{Not: &types.WhereExpr{And: types.WhereExpr2{
				L: &types.WhereExpr{Contains: types.WhereExpr2{
					L: &types.WhereExpr{Field: "participants"},
					R: &types.WhereExpr{Literal: "test"},
				}},
				R: &types.WhereExpr{Equals: types.WhereExpr2{
					L: &types.WhereExpr{Field: "login"},
					R: &types.WhereExpr{Literal: "root"},
				}}}}}))
	t.Run("or-condition, with identifier (negated)",
		testCase(`!(contains(session.participants, "test") || equals(session.login, "root"))`,
			types.WhereExpr{Not: &types.WhereExpr{Or: types.WhereExpr2{
				L: &types.WhereExpr{Contains: types.WhereExpr2{
					L: &types.WhereExpr{Field: "participants"},
					R: &types.WhereExpr{Literal: "test"},
				}},
				R: &types.WhereExpr{Equals: types.WhereExpr2{
					L: &types.WhereExpr{Field: "login"},
					R: &types.WhereExpr{Literal: "root"},
				}}}}}))

	t.Run("and-condition, mixed with and without identifier",
		testCase(`contains(session.participants, "test") && equals(user.metadata.name, "test-user")`,
			types.WhereExpr{Contains: types.WhereExpr2{
				L: &types.WhereExpr{Field: "participants"},
				R: &types.WhereExpr{Literal: "test"},
			}}))
	t.Run("and-condition, mixed with and without identifier (negated)",
		testCase(`!(contains(session.participants, "test") && equals(user.metadata.name, "test-user"))`,
			types.WhereExpr{Not: &types.WhereExpr{Contains: types.WhereExpr2{
				L: &types.WhereExpr{Field: "participants"},
				R: &types.WhereExpr{Literal: "test"},
			}}}))
	t.Run("and-condition, mixed with and without identifier (double negated)",
		testCase(`!!(contains(session.participants, "test") && equals(user.metadata.name, "test-user"))`,
			types.WhereExpr{Not: &types.WhereExpr{Not: &types.WhereExpr{Contains: types.WhereExpr2{
				L: &types.WhereExpr{Field: "participants"},
				R: &types.WhereExpr{Literal: "test"},
			}}}}))
	t.Run("and-condition, mixed with and without identifier (false)",
		testCase(`contains(session.participants, "test") && !equals(user.metadata.name, "test-user")`,
			types.WhereExpr{Literal: false}))
	t.Run("and-condition, mixed with and without identifier (negated false)",
		testCase(`!(contains(session.participants, "test") && !equals(user.metadata.name, "test-user"))`,
			types.WhereExpr{Literal: true}))

	t.Run("or-condition, mixed with and without identifier (true)",
		testCase(`contains(session.participants, "test") || !!equals(user.metadata.name, "test-user")`,
			types.WhereExpr{Literal: true}))
	t.Run("or-condition, mixed with and without identifier (negated true)",
		testCase(`!(contains(session.participants, "test") || equals(user.metadata.name, "test-user"))`,
			types.WhereExpr{Literal: false}))
	t.Run("or-condition, mixed with and without identifier (false)",
		testCase(`contains(session.participants, "test") || !equals(user.metadata.name, "test-user")`,
			types.WhereExpr{Contains: types.WhereExpr2{
				L: &types.WhereExpr{Field: "participants"},
				R: &types.WhereExpr{Literal: "test"},
			}}))

	t.Run("complex condition",
		testCase(`(contains(session.participants, "test1") && (contains(session.participants, "test2") || equals(user.metadata.name, "test-user"))) || (equals(session.login, "root") && contains(session.participants, "test3") && !equals(user.metadata.name, "test-user2")) || (contains(session.participants, "test4") && equals(user.metadata.name, "test-user3"))`,
			types.WhereExpr{Or: types.WhereExpr2{
				L: &types.WhereExpr{Contains: types.WhereExpr2{
					L: &types.WhereExpr{Field: "participants"},
					R: &types.WhereExpr{Literal: "test1"},
				}},
				R: &types.WhereExpr{And: types.WhereExpr2{
					L: &types.WhereExpr{Equals: types.WhereExpr2{
						L: &types.WhereExpr{Field: "login"},
						R: &types.WhereExpr{Literal: "root"},
					}},
					R: &types.WhereExpr{Contains: types.WhereExpr2{
						L: &types.WhereExpr{Field: "participants"},
						R: &types.WhereExpr{Literal: "test3"},
					}}}}}}))
}
