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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/predicate"

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
