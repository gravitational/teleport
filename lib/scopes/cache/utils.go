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
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/scopes"
)

// cursorV1Prefix is used to ensure that future changes in cursor format don't need to worry
// about causing unintended behaviors of inadvertently passed to older instances.
const cursorV1Prefix = "v1:"

// DecodeStringCursor decodes a cursor of the form `v1:<key>@<scope>` into a Cursor[string] struct. This is a reasonable
// default encoding for page tokens for string-keyed scoped items, so long as the key does not contain the '@' character.
func DecodeStringCursor(cursor string) (Cursor[string], error) {
	if cursor == "" {
		return Cursor[string]{}, nil
	}

	if !strings.HasPrefix(cursor, cursorV1Prefix) {
		return Cursor[string]{}, trace.BadParameter("cursor %q is not a valid v1 cursor", cursor)
	}

	var out Cursor[string]
	var n int
	for part := range strings.SplitSeq(strings.TrimPrefix(cursor, cursorV1Prefix), "@") {
		n++
		switch n {
		case 1:
			out.Key = part
		case 2:
			out.Scope = part
		default:
			return Cursor[string]{}, trace.BadParameter("too many parts in cursor: %q", cursor)
		}
	}

	if out.Key == "" || out.Scope == "" {
		return Cursor[string]{}, trace.BadParameter("cursor %q is missing key or scope", cursor)
	}

	if err := scopes.WeakValidate(out.Scope); err != nil {
		return Cursor[string]{}, trace.BadParameter("cursor %q has invalid scope %q: %v", cursor, out.Scope, err)
	}

	return out, nil
}

// EncodeStringCursor encodes a cursor struct into a string of the form `v1:<key>@<scope>`. This is a reasonable
// default encoding for page tokens for string-keyed scoped items, so long as the key does not contain the '@' character.
func EncodeStringCursor(cursor Cursor[string]) (string, error) {
	if cursor.Scope == "" && cursor.Key == "" {
		return "", nil
	}

	if cursor.Scope == "" || cursor.Key == "" {
		return "", trace.BadParameter("cursor key and scope must be non-empty")
	}

	if strings.Contains(cursor.Key, "@") {
		return "", trace.BadParameter("cursor key %q contains invalid character '@'", cursor.Key)
	}

	return cursorV1Prefix + cursor.Key + "@" + cursor.Scope, nil
}
