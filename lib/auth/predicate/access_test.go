/*
Copyright 2022 Gravitational, Inc.

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

package predicate

import (
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/stretchr/testify/require"
)

type fnSanityCase struct {
	name string
	expr string
}

// TestFnSanity tests that the execution environment including builtin functions are sane.
func TestFnSanity(t *testing.T) {
	cases := []fnSanityCase{
		{
			name: "add+eq",
			expr: "add(\"foo\", \"bar\") == \"foobar\"",
		},
		{
			name: "sub+eq",
			expr: "sub(5, 2) == 3",
		},
		{
			name: "mul+eq",
			expr: "mul(10, 5) == 50",
		},
		{
			name: "div+eq",
			expr: "div(20, 4) == 5",
		},
		{
			name: "and+eq+not",
			expr: "!(true && true == true && false)",
		},
		{
			name: "or",
			expr: "false || true",
		},
		{
			name: "lt+gt+and",
			expr: "5 < 6 && 6 > 5",
		},
		{
			name: "le+ge+and",
			expr: "6 <= 6 && 6 >= 6",
		},
		{
			name: "split+array+eq",
			expr: "split(\"foo,bar,baz\", \",\") == array(\"foo\", \"bar\", \"baz\")",
		},
		{
			name: "upper+eq",
			expr: "upper(\"foo\") == \"FOO\"",
		},
		{
			name: "lower+eq",
			expr: "lower(\"FoO\") == \"foo\"",
		},
		{
			name: "contains",
			expr: "contains(\"foobar\", \"foo\")",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			parser, err := newParser(nil)
			require.NoError(t, err)

			ifn, err := parser.Parse(c.expr)
			require.NoError(t, err)

			b, ok := ifn.(bool)
			require.True(t, ok)
			require.True(t, b)
		})
	}
}

// TestCheckAccessToNode checks that `CheckAccessToNode` works properly.
func TestCheckAccessToNode(t *testing.T) {
	withNameAsLogin := types.NewPolicy("allow", types.AccessPolicySpecV1{
		Allow: map[string]string{
			"node": "(node.login == user.name) || (add(user.name, \"-admin\") == node.login)",
		},
	})

	denyMike := types.NewPolicy("allow", types.AccessPolicySpecV1{
		Deny: map[string]string{
			"node": "node.login == \"mike\"",
		},
	})

	checker := NewPredicateAccessChecker([]types.Policy{withNameAsLogin})
	access, err := checker.CheckAccessToNode(&Node{Login: "mike"}, &User{Name: "mike"})
	require.NoError(t, err)
	require.True(t, access)

	access, err = checker.CheckAccessToNode(&Node{Login: "alice"}, &User{Name: "bob"})
	require.NoError(t, err)
	require.False(t, access)

	access, err = checker.CheckAccessToNode(&Node{Login: "bob-admin"}, &User{Name: "bob"})
	require.NoError(t, err)
	require.True(t, access)

	checkerWithDeny := NewPredicateAccessChecker([]types.Policy{withNameAsLogin, denyMike})
	access, err = checkerWithDeny.CheckAccessToNode(&Node{Login: "mike"}, &User{Name: "mike"})
	require.NoError(t, err)
	require.False(t, access)

	access, err = checkerWithDeny.CheckAccessToNode(&Node{Login: "bob"}, &User{Name: "bob"})
	require.NoError(t, err)
	require.True(t, access)
}
