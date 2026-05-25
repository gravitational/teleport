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

package common

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
)

func Test_beamsAddCommand_shouldShowSpinner(t *testing.T) {
	fakeTTY := func(io.Writer) bool { return true }
	fakeNonTTY := func(io.Writer) bool { return false }

	tests := []struct {
		name                string
		format              string
		isTerminalOverwrite func(io.Writer) bool
		check               require.BoolAssertionFunc
	}{
		{
			name:                "json format with TTY",
			format:              teleport.JSON,
			isTerminalOverwrite: fakeTTY,
			check:               require.False,
		},
		{
			name:                "yaml format with TTY",
			format:              teleport.YAML,
			isTerminalOverwrite: fakeTTY,
			check:               require.False,
		},
		{
			name:                "text format with TTY",
			format:              teleport.Text,
			isTerminalOverwrite: fakeTTY,
			check:               require.True,
		},
		{
			name:                "text format with non-TTY",
			format:              teleport.Text,
			isTerminalOverwrite: fakeNonTTY,
			check:               require.False,
		},
		{
			name:                "empty format with TTY",
			format:              "",
			isTerminalOverwrite: fakeTTY,
			check:               require.True,
		},
		{
			name:                "empty format with non-TTY",
			format:              "",
			isTerminalOverwrite: fakeNonTTY,
			check:               require.False,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &beamsAddCommand{
				isTerminalOverwrite: tt.isTerminalOverwrite,
			}
			tt.check(t, cmd.shouldShowSpinner(io.Discard, tt.format))
		})
	}
}
