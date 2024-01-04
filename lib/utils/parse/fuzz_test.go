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

package parse

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzNewExpression(f *testing.F) {
	f.Add("")
	f.Add("foo")
	f.Add("{{external.foo}}")
	f.Add(`{{regexp.replace(internal.foo, "foo-(.*)-(.*)", "$1.$2")}}`)

	f.Fuzz(func(t *testing.T, variable string) {
		require.NotPanics(t, func() {
			NewTraitsTemplateExpression(variable)
		})
	})
}

func FuzzNewMatcher(f *testing.F) {
	f.Add("")
	f.Add("foo")
	f.Add("*")
	f.Add("^foo$")
	f.Add(`{{regexp.match("foo.*")}}`)

	f.Fuzz(func(t *testing.T, value string) {
		require.NotPanics(t, func() {
			NewMatcher(value)
		})
	})
}
