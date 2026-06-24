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

// TestTerminalsTakeNoArguments pins that greedy() and slash() are terminal
// matchers: each compiles with an empty argument list and is a load error when
// given any child, rather than silently dropping it.
func TestTerminalsTakeNoArguments(t *testing.T) {
	for _, pred := range []string{
		`path.match(literal("files", greedy()))`,
		`path.match(literal("files", slash()))`,
	} {
		_, err := compileExpression(pred)
		require.NoError(t, err, pred)
	}

	for _, pred := range []string{
		`path.match(literal("files", greedy(literal("x"))))`,
		`path.match(literal("files", slash(literal("x"))))`,
	} {
		_, err := compileExpression(pred)
		require.ErrorContains(t, err, "accepts 0 arguments", pred)
	}
}
