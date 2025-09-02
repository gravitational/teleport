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
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/predicate"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

func TestParserForIdentifierSubcondition(t *testing.T) {
	t.Parallel()
	user, err := types.NewUser("test-user")
	require.NoError(t, err)
	user.SetTraits(map[string][]string{
		"test": {"value1", "value2"},
	})
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
				}},
			}}))
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
				}},
			}}}))
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
				}},
			}}}))

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
					}},
				}},
			}}))
}

func TestNewResourceExpression(t *testing.T) {
	t.Parallel()
	resource, err := types.NewServerWithLabels("test-name", types.KindNode, types.ServerSpecV2{
		Hostname: "test-hostname",
		Addr:     "test-addr",
		CmdLabels: map[string]types.CommandLabelV2{
			"version": {
				Result: "v8",
			},
		},
	}, map[string]string{
		"env": "prod",
		"os":  "mac",
	})
	require.NoError(t, err)

	t.Run("matching expressions", func(t *testing.T) {
		t.Parallel()
		exprs := []string{
			// Test equals.
			"equals(name, `test-hostname`)",
			`equals(resource.metadata.name, "test-name")`,
			`equals(labels.env, "prod")`,
			`equals(labels["env"], "prod")`,
			`equals(resource.metadata.labels["env"], "prod")`,
			`!equals(labels.env, "_")`,
			`!equals(labels.undefined, "prod")`,
			`equals(resource.spec.hostname, "test-hostname")`,
			`equals(health.status, "")`,
			// Test search.
			`search("mac")`,
			`search("os", "mac", "prod")`,
			`search()`,
			`!search("_")`,
			// Test hasPrefix.
			`hasPrefix(name, "")`,
			`hasPrefix(name, "test-h")`,
			`!hasPrefix(name, "foo")`,
			`hasPrefix(resource.metadata.labels["env"], "pro")`,
			// Test exists.
			`exists(labels.env)`,
			`!exists(labels.undefined)`,
			// Test identifiers outside call expressions.
			`resource.metadata.labels["env"] == "prod"`,
			"resource.metadata.labels[`env`] != `_`",
			`labels.env == "prod"`,
			`labels["env"] == "prod"`,
			`labels["env"] != "_"`,
			`name == "test-hostname"`,
			`health.status == ""`,
			// Test combos.
			`labels.os == "mac" && name == "test-hostname" && search("v8")`,
			`exists(labels.env) && labels["env"] != "qa"`,
			`search("does", "not", "exist") || resource.spec.addr == "_" || labels.version == "v8"`,
			`hasPrefix(labels.os, "m") && !hasPrefix(labels.env, "dev") && name == "test-hostname" && health.status != "healthy"`,
			// Test operator precedence
			`exists(labels.env) || (exists(labels.os) && labels.os != "mac")`,
			`exists(labels.env) || exists(labels.os) && labels.os != "mac"`,
		}
		for _, expr := range exprs {
			t.Run(expr, func(t *testing.T) {
				parser, err := NewResourceExpression(expr)
				require.NoError(t, err)

				match, err := parser.Evaluate(resource)
				require.NoError(t, err)
				require.True(t, match)
			})
		}
	})

	t.Run("non matching expressions", func(t *testing.T) {
		t.Parallel()
		exprs := []string{
			`(exists(labels.env) || exists(labels.os)) && labels.os != "mac"`,
			`exists(labels.undefined)`,
			`!exists(labels.env)`,
			`labels.env != "prod"`,
			`!equals(labels.env, "prod")`,
			`equals(resource.metadata.labels["undefined"], "prod")`,
			`name == "test"`,
			`health.status == "healthy"`,
			`equals(labels["env"], "wrong-value")`,
			`equals(resource.metadata.labels["env"], "wrong-value")`,
			`equals(resource.spec.hostname, "wrong-value")`,
			`search("mac", "not-found")`,
			`hasPrefix(name, "x")`,
		}
		for _, expr := range exprs {
			t.Run(expr, func(t *testing.T) {
				parser, err := NewResourceExpression(expr)
				require.NoError(t, err)

				match, err := parser.Evaluate(resource)
				require.NoError(t, err)
				require.False(t, match)
			})
		}
	})

	t.Run("fail to parse", func(t *testing.T) {
		t.Parallel()
		exprs := []string{
			`!name`,
			`name ==`,
			`name &`,
			`name &&`,
			`name ||`,
			`name |`,
			`&&`,
			`!`,
			`||`,
			`|`,
			`&`,
			`.`,
			`equals(invalidIdentifier)`,
			`equals(labels.env)`,
			`equals(labels.env, "too", "many")`,
			`equals()`,
			`exists()`,
			`exists(labels.env, "too", "many")`,
			`search(1,2)`,
			`"just-string"`,
			`hasPrefix(1, 2)`,
			`hasPrefix(name)`,
			`hasPrefix(name, 1)`,
			`hasPrefix(name, "too", "many")`,
			"",
		}
		for _, expr := range exprs {
			t.Run(expr, func(t *testing.T) {
				expression, err := NewResourceExpression(expr)
				require.Error(t, err)
				require.Nil(t, expression)
			})
		}
	})

	t.Run("fail to evaluate", func(t *testing.T) {
		t.Parallel()
		exprs := []string{
			`name.toomanyfield`,
			`labels.env.toomanyfield`,
			`equals(resource.incorrect.selector, "_")`,
		}
		for _, expr := range exprs {
			t.Run(expr, func(t *testing.T) {
				parser, err := NewResourceExpression(expr)
				require.NoError(t, err)

				match, err := parser.Evaluate(resource)
				require.Error(t, err)
				require.False(t, match)
			})
		}
	})
}

func TestResourceExpression_NameIdentifier(t *testing.T) {
	t.Parallel()

	// Server resource should use hostname when using name identifier.
	server, err := types.NewServerWithLabels("server-name", types.KindNode, types.ServerSpecV2{
		Hostname: "server-hostname",
	}, nil)
	require.NoError(t, err)

	parser, err := NewResourceExpression(`name == "server-hostname"`)
	require.NoError(t, err)

	match, err := parser.Evaluate(server)
	require.NoError(t, err)
	require.True(t, match)

	// Other resource types should use the default metadata name.
	desktop, err := types.NewWindowsDesktopV3("desktop-name", nil, types.WindowsDesktopSpecV3{
		Addr: "some-address",
	})
	require.NoError(t, err)

	parser, err = NewResourceExpression(`name == "desktop-name"`)
	require.NoError(t, err)

	match, err = parser.Evaluate(desktop)
	require.NoError(t, err)
	require.True(t, match)
}

func TestResourceParserLabelExpansion(t *testing.T) {
	t.Parallel()

	// Server resource should use hostname when using name identifier.
	server, err := types.NewServerWithLabels("server-name", types.KindNode, types.ServerSpecV2{
		Hostname: "server-hostname",
	}, map[string]string{"ip": "1.2.3.11,1.2.3.101,1.2.3.1", "foo": "bar"})
	require.NoError(t, err)

	tests := []struct {
		expression string
		assertion  require.BoolAssertionFunc
	}{
		{
			expression: `contains(split(labels["ip"], ","), "1.2.3.1")`,
			assertion:  require.True,
		},
		{
			expression: `contains(split(labels.ip, ","), "1.2.3.1",)`,
			assertion:  require.True,
		},
		{
			expression: `contains(split(labels["ip"], ","),  "1.2.3.2")`,
			assertion:  require.False,
		},
		{
			expression: `contains(split(labels.llama, ","),  "1.2.3.2")`,
			assertion:  require.False,
		},
		{
			expression: `contains(split(labels.ip, ","), "1.2.3.2")`,
			assertion:  require.False,
		},
		{
			expression: `contains(split(labels.foo, ","), "bar")`,
			assertion:  require.True,
		},
	}

	for _, test := range tests {
		t.Run(test.expression, func(t *testing.T) {
			expression, err := NewResourceExpression(test.expression)
			require.NoError(t, err)

			match, err := expression.Evaluate(server)
			require.NoError(t, err)
			test.assertion(t, match)
		})
	}
}

func BenchmarkContains(b *testing.B) {
	server, err := types.NewServerWithLabels("server-name", types.KindNode, types.ServerSpecV2{
		Hostname: "server-hostname",
	}, map[string]string{"ip": "1.2.3.11|1.2.3.101|1.2.3.1"})
	require.NoError(b, err)

	expression, err := NewResourceExpression(`contains(split(labels["ip"], "|"), "1.2.3.1")`)
	require.NoError(b, err)

	for b.Loop() {
		match, err := expression.Evaluate(server)
		require.NoError(b, err)
		require.True(b, match)
	}
}

// TestParserHostCertContext tests set functions with a custom host cert
// context.
func TestParserHostCertContext(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		desc       string
		principals []string
		positive   []string
		negative   []string
	}{
		{
			desc:       "simple",
			principals: []string{"foo.example.com"},
			positive: []string{
				`all_equal(host_cert.principals, "foo.example.com")`,
				`is_subset(host_cert.principals, "a", "b", "foo.example.com")`,
				`all_end_with(host_cert.principals, ".example.com")`,
				`contains_any(set("a", "b", "foo.example.com"), host_cert.principals)`,
			},
			negative: []string{
				`all_equal(host_cert.principals, "foo")`,
				`is_subset(host_cert.principals, "a", "b", "c")`,
				`all_end_with(host_cert.principals, ".foo")`,
			},
		},
		{
			desc:       "complex",
			principals: []string{"node.foo.example.com", "node.bar.example.com"},
			positive: []string{
				`all_end_with(host_cert.principals, ".example.com")`,
				`all_end_with(host_cert.principals, ".example.com") && !all_end_with(host_cert.principals, ".baz.example.com")`,
				`equals(host_cert.host_id, "") && is_subset(host_cert.principals, "node.bar.example.com", "node.foo.example.com", "node.baz.example.com")`,
			},
			negative: []string{
				`all_equal(host_cert.principals, "node.foo.example.com")`,
				`all_end_with(host_cert.principals, ".foo.example.com") || all_end_with(host_cert.principals, ".bar.example.com")`,
				`is_subset(host_cert.principals, "node.bar.example.com")`,
			},
		},
	} {
		ctx := Context{
			User: &types.UserV2{},
			HostCert: &HostCertContext{
				HostID:      "",
				NodeName:    "foo",
				Principals:  test.principals,
				ClusterName: "example.com",
				Role:        types.RoleNode,
				TTL:         time.Minute * 20,
			},
		}
		parser, err := NewWhereParser(&ctx)
		require.NoError(t, err)

		t.Run(test.desc, func(t *testing.T) {
			t.Run("positive", func(t *testing.T) {
				for _, pred := range test.positive {
					expr, err := parser.Parse(pred)
					require.NoError(t, err)

					ret, ok := expr.(predicate.BoolPredicate)
					require.True(t, ok)

					require.True(t, ret(), pred)
				}
			})

			t.Run("negative", func(t *testing.T) {
				for _, pred := range test.negative {
					expr, err := parser.Parse(pred)
					require.NoError(t, err)

					ret, ok := expr.(predicate.BoolPredicate)
					require.True(t, ok)

					require.False(t, ret(), pred)
				}
			})
		})
	}
}

func TestPredicateContainsAll(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{
			name: "both empty",
			a:    []string{},
			b:    []string{},
			want: false,
		},
		{
			name: "a empty, b not",
			a:    []string{},
			b:    []string{"a"},
			want: false,
		},
		{
			name: "a not empty, b empty",
			a:    []string{"a"},
			b:    []string{},
			want: false,
		},
		{
			name: "a contains b",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "c"},
			want: true,
		},
		{
			name: "a does not contain b",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "d"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := predicateContainsAll(tt.a, tt.b)
			require.Equal(t, tt.want, got())
		})
	}
}

func TestPredicateContainsAny(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{
			name: "both empty",
			a:    []string{},
			b:    []string{},
			want: false,
		},
		{
			name: "a empty, b not",
			a:    []string{},
			b:    []string{"a"},
			want: false,
		},
		{
			name: "a not empty, b empty",
			a:    []string{"a"},
			b:    []string{},
			want: false, // no elements in b to be contained in a
		},
		{
			name: "a contains b",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "c"},
			want: true,
		},
		{
			name: "a does not contain b",
			a:    []string{"a", "b", "c"},
			b:    []string{"d", "e"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := predicateContainsAny(tt.a, tt.b)
			require.Equal(t, tt.want, got())
		})
	}
}

func TestCanView(t *testing.T) {
	newRole := func(mut func(*types.RoleV6)) *types.RoleV6 {
		r := newRole(mut)
		r.Spec.Allow.Rules = append(r.Spec.Allow.Rules, types.Rule{
			Resources: []string{types.KindSession},
			Verbs:     []string{types.VerbRead, types.VerbList},
			Where:     "can_view()",
		})
		r.CheckAndSetDefaults()
		return r
	}

	user, err := types.NewUser("test-user")
	require.NoError(t, err)
	type check struct {
		server    types.Server
		hasAccess bool
	}
	serverNoLabels := &types.ServerV2{
		Kind: types.KindNode,
		Metadata: types.Metadata{
			Name: "a",
		},
	}
	serverWorker := &types.ServerV2{
		Kind: types.KindNode,
		Metadata: types.Metadata{
			Name:      "b",
			Namespace: apidefaults.Namespace,
			Labels:    map[string]string{"role": "worker", "status": "follower"},
		},
	}
	namespaceC := "namespace-c"
	serverDB := &types.ServerV2{
		Kind: types.KindNode,
		Metadata: types.Metadata{
			Name:      "c",
			Namespace: namespaceC,
			Labels:    map[string]string{"role": "db", "status": "follower"},
		},
	}
	serverDBWithSuffix := &types.ServerV2{
		Kind: types.KindNode,
		Metadata: types.Metadata{
			Name:      "c2",
			Namespace: namespaceC,
			Labels:    map[string]string{"role": "db01", "status": "follower01"},
		},
	}
	testCases := []struct {
		name   string
		roles  []*types.RoleV6
		checks []check
	}{
		{
			name:  "empty role set has access to nothing",
			roles: []*types.RoleV6{},
			checks: []check{
				{server: serverNoLabels, hasAccess: false},
				{server: serverWorker, hasAccess: false},
				{server: serverDB, hasAccess: false},
			},
		},
		{
			name: "role is limited to labels in default namespace",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"admin"}
					r.Spec.Allow.NodeLabels = types.Labels{"role": []string{"worker"}}
				}),
			},
			checks: []check{
				{server: serverNoLabels, hasAccess: false},
				{server: serverWorker, hasAccess: true},
				{server: serverDB, hasAccess: false},
			},
		},
		{
			name: "role matches any label out of multiple labels",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"admin"}
					r.Spec.Allow.NodeLabels = types.Labels{"role": []string{"worker2", "worker"}}
				}),
			},
			checks: []check{
				{server: serverNoLabels, hasAccess: false},
				{server: serverWorker, hasAccess: true},
				{server: serverDB, hasAccess: false},
			},
		},
		{
			name: "node_labels with empty list value matches nothing",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"admin"}
					r.Spec.Allow.NodeLabels = types.Labels{"role": []string{}}
				}),
			},
			checks: []check{
				{server: serverNoLabels, hasAccess: false},
				{server: serverWorker, hasAccess: false},
				{server: serverDB, hasAccess: false},
			},
		},
		{
			name: "one role is more permissive than another",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"admin"}
					r.Spec.Allow.Namespaces = []string{apidefaults.Namespace}
					r.Spec.Allow.NodeLabels = types.Labels{"role": []string{"worker"}}
				}),
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root", "admin"}
				}),
			},
			checks: []check{
				{server: serverNoLabels, hasAccess: true},
				{server: serverWorker, hasAccess: true},
				{server: serverDB, hasAccess: true},
			},
		},
		{
			name: "one role needs to access servers sharing the partially same label value",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"admin"}
					r.Spec.Allow.NodeLabels = types.Labels{"role": []string{"^db(.*)$"}, "status": []string{"follow*"}}
					r.Spec.Allow.Namespaces = []string{namespaceC}
				}),
			},
			checks: []check{
				{server: serverNoLabels, hasAccess: false},
				{server: serverWorker, hasAccess: false},
				{server: serverDB, hasAccess: true},
				{server: serverDBWithSuffix, hasAccess: true},
			},
		},
		{
			name: "no logins means  access",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = nil
				}),
			},
			checks: []check{
				{server: serverNoLabels, hasAccess: true},
				{server: serverWorker, hasAccess: true},
				{server: serverDB, hasAccess: true},
			},
		},
		// MFA.
		{
			name: "one role requires MFA but MFA was not verified",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Allow.NodeLabels = types.Labels{"role": []string{"worker"}}
					r.Spec.Options.RequireMFAType = types.RequireMFAType_SESSION
				}),
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Options.RequireMFAType = types.RequireMFAType_OFF
				}),
			},
			checks: []check{
				{server: serverNoLabels, hasAccess: true},
				{server: serverWorker, hasAccess: true},
				{server: serverDB, hasAccess: true},
			},
		},

		// Device Trust.
		{
			name: "role requires trusted device, device not verified",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Options.DeviceTrustMode = constants.DeviceTrustModeRequired
				}),
			},
			checks: []check{
				{server: serverNoLabels, hasAccess: true},
				{server: serverWorker, hasAccess: true},
				{server: serverDB, hasAccess: true},
			},
		},
		{
			name: "label expressions",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.NodeLabels = nil
					r.Spec.Allow.NodeLabelsExpression = `labels.role == "worker" && labels.status == "follower"`
					r.Spec.Allow.Logins = []string{"root"}
				}),
			},
			checks: []check{
				{server: serverNoLabels, hasAccess: false},
				{server: serverWorker, hasAccess: true},
				{server: serverDB, hasAccess: false},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			accessChecker := makeAccessCheckerWithRolePointers(tc.roles)
			for j, check := range tc.checks {
				comment := fmt.Sprintf("check #%v:  server: %v, should access: %v", j, check.server.GetName(), check.hasAccess)
				serviceCtx := &Context{
					User:          user,
					AccessChecker: accessChecker,
					Resource:      check.server,
				}
				err := accessChecker.CheckAccessToRule(
					serviceCtx,
					check.server.GetNamespace(),
					types.KindSession,
					types.VerbRead,
				)
				if check.hasAccess {
					require.NoError(t, err, comment)
				} else {
					require.True(t, trace.IsAccessDenied(err), "Got err = %v/%T, wanted AccessDenied. %v", err, err, comment)
				}
			}
		})
	}
}
