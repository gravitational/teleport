/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package expression_test

import (
	"testing"

	"github.com/gravitational/teleport/lib/auth/machineid/workloadidentityv1/expression"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSigstorePolicyNames(t *testing.T) {
	testCases := map[string]struct {
		expr  string
		names []string
	}{
		"no calls": {
			expr:  `1 == 1`,
			names: []string{},
		},
		"simple call": {
			expr:  `sigstore.policy_satisfied("foo")`,
			names: []string{"foo"},
		},
		"many calls": {
			expr:  `sigstore.policy_satisfied("a") || sigstore.policy_satisfied("b", "c") && sigstore.policy_satisfied("d")`,
			names: []string{"a", "b", "c", "d"},
		},
		"parens": {
			expr:  `(sigstore.policy_satisfied("foo"))`,
			names: []string{"foo"},
		},
		"non-literal arguments": {
			expr:  `sigstore.policy_satisfied(some_var, some_func())`,
			names: []string{},
		},
		"called as a sub-expression": {
			expr:  `some_func(sigstore.policy_satisfied("a"), sigstore.policy_satisfied("b"))`,
			names: []string{"a", "b"},
		},
		"used to index an array or map": {
			expr:  `some_array_or_map[sigstore.policy_satisfied("foo")]`,
			names: []string{"foo"},
		},
		"unary expression": {
			expr:  `!sigstore.policy_satisfied("foo")`,
			names: []string{"foo"},
		},
		"complex expression": {
			expr:  `a.b.c == "d" || (some_func(sigstore.policy_satisfied("a") || sigstore.policy_satisfied("b")) && sigstore.policy_satisfied("c", "d", 1234, true, "e")).property || foo["bar"]`,
			names: []string{"a", "b", "c", "d", "e"},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			names, err := expression.SigstorePolicyNames(tc.expr)
			require.NoError(t, err)
			assert.ElementsMatch(t, tc.names, names)
		})
	}
}
