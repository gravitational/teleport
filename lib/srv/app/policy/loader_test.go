/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package policy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompile_AcceptsBothAllowAndDeny(t *testing.T) {
	p, err := Compile(Spec{
		Name:  "mixed",
		Allow: []RuleSpec{{Paths: []string{"/foo"}}},
		Deny:  []RuleSpec{{Paths: []string{"/bar"}}},
	})
	require.NoError(t, err)
	require.Len(t, p.Allow, 1)
	require.Len(t, p.Deny, 1)
	require.Equal(t, "mixed", p.Allow[0].ReasonCode)
	require.Equal(t, ReasonExplicitDeny, p.Deny[0].ReasonCode)
}

func TestCompile_RejectsEmpty(t *testing.T) {
	_, err := Compile(Spec{Name: "empty"})
	require.Error(t, err)
}

func TestCompile_AcceptsMultipleDeniesWithDefaultCode(t *testing.T) {
	p, err := Compile(Spec{
		Name: "many-denies",
		Deny: []RuleSpec{
			{Paths: []string{"/a"}},
			{Paths: []string{"/b"}},
		},
	})
	require.NoError(t, err)
	require.Equal(t, ReasonExplicitDeny, p.Deny[0].ReasonCode)
	require.Equal(t, ReasonExplicitDeny, p.Deny[1].ReasonCode)
}

func TestCompile_RejectsAllowWithoutPaths(t *testing.T) {
	_, err := Compile(Spec{
		Name:  "where-only",
		Allow: []RuleSpec{{Where: `user.name == "alice"`}},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "allow rule must set paths")
}

func TestCompile_RejectsReservedReasonCodePrefix(t *testing.T) {
	_, err := Compile(Spec{
		Name:  "blocked",
		Allow: []RuleSpec{{Paths: []string{"/foo"}, ReasonCode: "teleport_custom"}},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "teleport_")
}

func TestCompile_RejectsReservedNamePrefix(t *testing.T) {
	_, err := Compile(Spec{
		Name:  "teleport_custom",
		Allow: []RuleSpec{{Paths: []string{"/foo"}}},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "teleport_")
}

func TestCompile_RejectsDuplicateReasonCodes(t *testing.T) {
	_, err := Compile(Spec{
		Name: "dup",
		Allow: []RuleSpec{
			{Paths: []string{"/a"}, ReasonCode: "same"},
			{Paths: []string{"/b"}, ReasonCode: "same"},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate")
}

// TestCompile_UpperCasesMethods locks in compile-time normalization
// of HTTP methods. The matcher uppercases the request method but
// compares against the stored Methods slice as-is, so a config of
// `methods: [post]` would silently fail to match POST requests
// without this normalization.
func TestCompile_UpperCasesMethods(t *testing.T) {
	p, err := Compile(Spec{
		Name: "deny-post",
		Deny: []RuleSpec{{
			Paths:   []string{"/admin"},
			Methods: []string{"post", "Delete"},
		}},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"POST", "DELETE"}, p.Deny[0].Methods)
	require.True(t, p.Deny[0].MatchesMethod("POST"))
	require.True(t, p.Deny[0].MatchesMethod("DELETE"))
}

func TestCompile_FillsDefaults(t *testing.T) {
	p, err := Compile(Spec{
		Name:  "default",
		Allow: []RuleSpec{{Paths: []string{"/foo"}}},
	})
	require.NoError(t, err)
	require.Equal(t, "default", p.Allow[0].ReasonCode)
	require.Equal(t, "default", p.Allow[0].Reason)
}

func TestCompileAll_RejectsDuplicateNames(t *testing.T) {
	_, err := CompileAll([]Spec{
		{Name: "p", Allow: []RuleSpec{{Paths: []string{"/a"}}}},
		{Name: "p", Allow: []RuleSpec{{Paths: []string{"/b"}}}},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate policy name")
}

func TestResolve_ByName(t *testing.T) {
	lib, err := CompileAll([]Spec{
		{Name: "read-only", Allow: []RuleSpec{{Paths: []string{"/**"}, Methods: []string{"GET"}}}},
	})
	require.NoError(t, err)

	out, err := Resolve(lib, []Ref{{Name: "read-only"}})
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "read-only", out[0].Name)
}

func TestResolve_UnknownName(t *testing.T) {
	_, err := Resolve(nil, []Ref{{Name: "missing"}})
	require.Error(t, err)
}

func TestResolve_Inline(t *testing.T) {
	out, err := Resolve(nil, []Ref{{Inline: &Spec{
		Name:  "inline",
		Allow: []RuleSpec{{Paths: []string{"/foo"}}},
	}}})
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "inline", out[0].Name)
}

func TestResolve_EmptyRef(t *testing.T) {
	_, err := Resolve(nil, []Ref{{}})
	require.Error(t, err)
}

func TestResolve_BothNameAndInline(t *testing.T) {
	_, err := Resolve(nil, []Ref{{
		Name:   "x",
		Inline: &Spec{Name: "x"},
	}})
	require.Error(t, err)
}
