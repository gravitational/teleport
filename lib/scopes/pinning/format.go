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
	"slices"
	"strings"

	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/lib/scopes"
)

// FormatAssignmentTree formats an assignment tree for command-line display in a structure superficially similar to
// the unix tree utility. The output shows scopes of origin as top-level nodes, with scopes of effect as child nodes
// displaying the roles assigned at each (origin, effect) pair. Both scopes of origin and scopes of effect are sorted
// using the scopes.Sort function to ensure consistent ordering. The prefix parameter is prepended to each
// line, allowing the output to be easily composed into larger outputs.
//
// Example output with prefix "":
//
//	├── / (root)
//	│   ├── /staging - staging-reader, staging-test
//	│   ├── /staging/west - staging-west-debug
//	│   └── /staging/east - staging-east-debug
//	├── /staging
//	│   ├── /staging/west - staging-auditor, staging-editor
//	│   └── /staging/east - staging-user
//	└── /staging/west
//	    └── /staging/west - staging-west-test
//
// Note that the output lends itself to placing a title in the preceding line, for example:
//
//	Scoped Roles:
//	├── / (root)
//	│   └── ...
//	└── ...
//
// This utility was written primarily for use in tsh when displaying scoped role assignments as part
// of 'tsh status', but may have other applications as well. Note that this utility does not perform
// validation and may produce confusing output if the input tree is malformed.
func FormatAssignmentTree(tree *scopesv1.AssignmentNode, prefix string) string {
	assignmentMap := AssignmentTreeIntoMap(tree)
	if len(assignmentMap) == 0 {
		return prefix + "└── (none)\n"
	}

	// collect and sort all scopes of origin first so that we can order them
	// and keep track of our position.
	var originsSlice []string
	for origin := range assignmentMap {
		originsSlice = append(originsSlice, origin)
	}
	slices.SortFunc(originsSlice, scopes.Sort)

	var b strings.Builder
	// format each scope of origin
	for i, origin := range originsSlice {
		isLastOrigin := i == len(originsSlice)-1

		// format the origin scope name
		originDisplay := origin
		if origin == scopes.Root {
			originDisplay = scopes.Root + " (root)"
		}

		// draw the outer branch for this scope of origin
		b.WriteString(prefix)
		if isLastOrigin {
			b.WriteString("└── ")
		} else {
			b.WriteString("├── ")
		}
		b.WriteString(originDisplay)
		b.WriteString("\n")

		// collect and sort all scopes of effect for this origin so we can order them
		// and keep track of our position.
		effectMap := assignmentMap[origin]
		var effectsSlice []string
		for effect := range effectMap {
			effectsSlice = append(effectsSlice, effect)
		}
		slices.SortFunc(effectsSlice, scopes.Sort)

		// format each scope of effect
		for j, effect := range effectsSlice {
			isLastEffect := j == len(effectsSlice)-1

			// draw the continuation and inner branch for this scope of effect
			b.WriteString(prefix)
			if isLastOrigin {
				b.WriteString("    ")
			} else {
				b.WriteString("│   ")
			}

			if isLastEffect {
				b.WriteString("└── ")
			} else {
				b.WriteString("├── ")
			}

			// write the scope of effect and role list
			// TODO(fspmarshall/scopes): should we truncate long role lists? similarly, should we try to pad out role lists to form a column?
			b.WriteString(effect)
			b.WriteString(" - ")
			roles := effectMap[effect]
			b.WriteString(strings.Join(roles, ", "))
			b.WriteString("\n")
		}
	}

	return b.String()
}
