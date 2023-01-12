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
	withNameAsLogin := types.NewAccessPolicy("allow", types.AccessPolicySpecV1{
		Allow: map[string]string{
			"access_node": "(access_node.login == user.name) || (add(user.name, \"-admin\") == access_node.login)",
		},
	})

	denyMike := types.NewAccessPolicy("deny", types.AccessPolicySpecV1{
		Deny: map[string]string{
			"access_node": "access_node.login == \"mike\"",
		},
	})

	checker := NewPredicateAccessChecker([]types.AccessPolicy{withNameAsLogin})
	access, err := checker.CheckLoginAccessToNode(&Node{}, &AccessNode{Login: "mike"}, &User{Name: "mike"})
	require.NoError(t, err)
	require.Equal(t, access, AccessAllowed)

	access, err = checker.CheckLoginAccessToNode(&Node{}, &AccessNode{Login: "alice"}, &User{Name: "bob"})
	require.NoError(t, err)
	require.Equal(t, access, AccessUndecided)

	access, err = checker.CheckLoginAccessToNode(&Node{}, &AccessNode{Login: "bob-admin"}, &User{Name: "bob"})
	require.NoError(t, err)
	require.Equal(t, access, AccessAllowed)

	checkerWithDeny := NewPredicateAccessChecker([]types.AccessPolicy{withNameAsLogin, denyMike})
	access, err = checkerWithDeny.CheckLoginAccessToNode(&Node{}, &AccessNode{Login: "mike"}, &User{Name: "mike"})
	require.NoError(t, err)
	require.Equal(t, access, AccessDenied)

	access, err = checkerWithDeny.CheckLoginAccessToNode(&Node{}, &AccessNode{Login: "bob"}, &User{Name: "bob"})
	require.NoError(t, err)
	require.Equal(t, access, AccessAllowed)
}
