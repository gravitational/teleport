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

package client

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTerminalCRLFWriter(t *testing.T) {
	t.Parallel()

	t.Run("converts LF to CRLF", func(t *testing.T) {
		var out bytes.Buffer
		w := &terminalCRLFWriter{writer: &out}

		n, err := w.Write([]byte("line 1\nline 2\n"))
		require.NoError(t, err)
		require.Equal(t, len("line 1\nline 2\n"), n)
		require.Equal(t, "line 1\r\nline 2\r\n", out.String())
	})

	t.Run("preserves existing CRLF", func(t *testing.T) {
		var out bytes.Buffer
		w := &terminalCRLFWriter{writer: &out}

		n, err := w.Write([]byte("line 1\r\nline 2\r\n"))
		require.NoError(t, err)
		require.Equal(t, len("line 1\r\nline 2\r\n"), n)
		require.Equal(t, "line 1\r\nline 2\r\n", out.String())
	})

	t.Run("preserves CRLF across write boundaries", func(t *testing.T) {
		var out bytes.Buffer
		w := &terminalCRLFWriter{writer: &out}

		n, err := w.Write([]byte("line 1\r"))
		require.NoError(t, err)
		require.Equal(t, len("line 1\r"), n)

		n, err = w.Write([]byte("\nline 2\n"))
		require.NoError(t, err)
		require.Equal(t, len("\nline 2\n"), n)

		require.Equal(t, "line 1\r\nline 2\r\n", out.String())
	})
}
