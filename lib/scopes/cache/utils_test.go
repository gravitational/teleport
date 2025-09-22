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

package cache

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringCursorFormat(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name   string
		cursor Cursor[string]
		enc    string
		ok     bool
	}{
		{
			name:   "empty",
			cursor: Cursor[string]{},
			enc:    "",
			ok:     true,
		},
		{
			name:   "root",
			cursor: Cursor[string]{Key: "foo", Scope: "/"},
			enc:    "v1:foo@/",
			ok:     true,
		},
		{
			name:   "non-root",
			cursor: Cursor[string]{Key: "foo", Scope: "/bar"},
			enc:    "v1:foo@/bar",
			ok:     true,
		},
		{
			name:   "empty scope",
			cursor: Cursor[string]{Key: "foo", Scope: ""},
			enc:    "v1:foo@",
			ok:     false,
		},
		{
			name:   "empty key",
			cursor: Cursor[string]{Key: "", Scope: "/bar"},
			enc:    "v1:@/bar",
			ok:     false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			enc, err := EncodeStringCursor(tt.cursor)
			if tt.ok {
				require.NoError(t, err)
				require.Equal(t, tt.enc, enc)
			} else {
				require.Error(t, err)
				require.Empty(t, enc)
			}

			cursor, err := DecodeStringCursor(tt.enc)
			if tt.ok {
				require.NoError(t, err)
				require.Equal(t, tt.cursor, cursor)
			} else {
				require.Error(t, err)
				require.Equal(t, Cursor[string]{}, cursor)
			}
		})
	}
}
