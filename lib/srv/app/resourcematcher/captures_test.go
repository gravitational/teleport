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

package resourcematcher

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCaptureLoadCheck pins that a rule reading a capture its matcher does not
// bind is rejected at compile time, in both the declarative and predicate
// surfaces. A bound capture, and a rule with no capture reads, compile fine.
func TestCaptureLoadCheck(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "sugared: where reads a bound capture",
			yaml: `
paths: ["/api/v4/projects/{project}/**"]
where: contains(user.traits["allowed_projects"], vars.project)
`,
		},
		{
			name: "sugared: where reads an unbound capture",
			yaml: `
paths: ["/api/v4/projects/{project}/**"]
where: contains(user.traits["allowed_projects"], vars.proejct)
`,
			wantErr: true,
		},
		{
			name: "sugared: where reads a capture with no path at all",
			yaml: `
methods: [GET]
where: vars.project == user.name
`,
			wantErr: true,
		},
		{
			name: "sugared: cross-path or, capture not on every path",
			yaml: `
paths: ["/project/{project}", "/user/{user}"]
where: vars.project == user.name || vars.user == user.name
`,
			wantErr: true,
		},
		{
			name: "sugared: capture path mixed with a captureless path",
			yaml: `
paths:
  - "/api/v4/projects/{project}/**"
  - "/api/status"
where: contains(user.traits["allowed_projects"], vars.project)
`,
			wantErr: true,
		},
		{
			name: "sugared: capture bound on every path",
			yaml: `
paths: ["/project/{project}/issues", "/project/{project}/merge_requests"]
where: vars.project == user.name
`,
		},
		{
			name: "predicate: reads a bound capture",
			yaml: `
pred: |
  path.match(literal("api", capture("project", greedy()))) &&
  contains(user.traits["allowed_projects"], vars.project)
`,
		},
		{
			name: "predicate: reads an unbound capture",
			yaml: `
pred: |
  path.match(literal("api", capture("project", greedy()))) &&
  vars.repo == user.name
`,
			wantErr: true,
		},
		{
			name: "no captures referenced",
			yaml: `
paths: ["/api/v4/user"]
methods: [GET]
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ruleFromYAML(t, tt.yaml).Compile()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestUnboundCaptureFailsClosed pins the runtime guard that backstops the load
// check. The load check enforces that every matching path binds a referenced
// capture, but it cannot see ordering: an author who reads vars.project before
// the path.match that binds it passes the load check, yet the read happens
// while the capture is still unbound. The guard forces the rule to deny in that
// case, regardless of the operator, so a bare vars.x != "..." on an empty value
// cannot widen.
func TestUnboundCaptureFailsClosed(t *testing.T) {
	// The read precedes the match. && evaluates the left side first, so
	// vars.project is read before path.match binds it.
	rule := ruleFromYAML(t, `
pred: |
  vars.project != "forbidden" &&
  path.match(literal("api", capture("project", greedy())))
`)
	compiled, err := rule.Compile()
	require.NoError(t, err)

	got, err := compiled.Evaluate(Request{Method: "GET", Path: "/api/allowed"}, Identity{})
	require.NoError(t, err)
	require.False(t, got.Allowed, "capture read before its match is unbound and must deny, not widen")
}
