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

package pinning

import (
	"testing"

	"github.com/stretchr/testify/require"

	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

// TestFormatAssignmentTree verifies that FormatAssignmentTree correctly formats assignment trees
// as tree-structured output with proper sorting and indentation.
func TestFormatAssignmentTree(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		tree   *scopesv1.AssignmentNode
		prefix string
	}{
		{
			name: "nil tree",
			tree: nil,
		},
		{
			name:   "nil tree with prefix",
			tree:   nil,
			prefix: "  ",
		},
		{
			name: "single origin single effect",
			tree: AssignmentTreeFromMap(map[string]map[string][]string{
				"/staging": {
					"/staging": {"admin", "user"},
				},
			}),
		},
		{
			name: "single origin multiple effects",
			tree: AssignmentTreeFromMap(map[string]map[string][]string{
				"/": {
					"/":             {"global"},
					"/staging":      {"staging-reader", "staging-test"},
					"/staging/west": {"staging-west-debug"},
					"/staging/east": {"staging-east-debug"},
				},
			}),
		},
		{
			name: "multiple origin and effect",
			tree: AssignmentTreeFromMap(map[string]map[string][]string{
				"/": {
					"/staging":      {"staging-reader", "staging-test"},
					"/staging/west": {"staging-west-debug"},
					"/staging/east": {"staging-east-debug"},
				},
				"/staging": {
					"/staging/west": {"staging-auditor", "staging-editor"},
					"/staging/east": {"staging-user"},
				},
				"/staging/west": {
					"/staging/west": {"staging-west-test"},
				},
			}),
		},
		{
			name: "with indentation",
			tree: AssignmentTreeFromMap(map[string]map[string][]string{
				"/": {
					"/staging": {"reader"},
				},
				"/staging": {
					"/staging": {"admin"},
				},
			}),
			prefix: "  ",
		},
		{
			name: "multi with orthogonals",
			tree: AssignmentTreeFromMap(map[string]map[string][]string{
				"/": {
					"/prod":         {"prod-global"},
					"/staging":      {"staging-global"},
					"/dev":          {"dev-global"},
					"/prod/east":    {"prod-east"},
					"/staging/west": {"staging-west"},
				},
				"/staging": {
					"/staging/west": {"west-admin"},
					"/staging/east": {"east-admin"},
				},
				"/prod": {
					"/prod/us": {"us-admin"},
					"/prod/eu": {"eu-admin"},
				},
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatted := FormatAssignmentTree(tt.tree, tt.prefix)
			if golden.ShouldSet() {
				golden.Set(t, []byte(formatted))
			}
			require.Equal(t, string(golden.Get(t)), formatted)
		})
	}
}
