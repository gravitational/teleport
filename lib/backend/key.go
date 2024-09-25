// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package backend

import (
	"bytes"
	"fmt"
	"strings"
)

// Key is the unique identifier for an [Item].
type Key []byte

// Separator is used as a separator between key parts
const Separator = '/'

// NewKey joins parts into path separated by Separator,
// makes sure path always starts with Separator ("/")
func NewKey(parts ...string) Key {
	return internalKey("", parts...)
}

// ExactKey is like Key, except a Separator is appended to the result
// path of Key. This is to ensure range matching of a path will only
// math child paths and not other paths that have the resulting path
// as a prefix.
func ExactKey(parts ...string) Key {
	return append(NewKey(parts...), Separator)
}

func internalKey(internalPrefix string, parts ...string) Key {
	return Key(strings.Join(append([]string{internalPrefix}, parts...), string(Separator)))
}

// String returns the textual representation of the key with
// each component concatenated together via the [Separator].
func (k Key) String() string {
	return string(k)
}

// IsZero reports whether k represents the zero key.
func (k Key) IsZero() bool {
	return len(k) == 0
}

// HasPrefix reports whether the key begins with prefix.
func (k Key) HasPrefix(prefix Key) bool {
	return bytes.HasPrefix(k, prefix)
}

// TrimPrefix returns the key without the provided leading prefix string.
// If the key doesn't start with prefix, it is returned unchanged.
func (k Key) TrimPrefix(prefix Key) Key {
	return bytes.TrimPrefix(k, prefix)
}

func (k Key) PrependPrefix(p Key) Key {
	return append(p, k...)
}

// HasSuffix reports whether the key ends with suffix.
func (k Key) HasSuffix(suffix Key) bool {
	return bytes.HasSuffix(k, suffix)
}

// TrimSuffix returns the key without the provided trailing suffix string.
// If the key doesn't end with suffix, it is returned unchanged.
func (k Key) TrimSuffix(suffix Key) Key {
	return bytes.TrimSuffix(k, suffix)
}

func (k Key) Components() [][]byte {
	if len(k) == 0 {
		return nil
	}

	sep := []byte{Separator}
	return bytes.Split(bytes.TrimPrefix(k, sep), sep)
}

func (k Key) Compare(o Key) int {
	return bytes.Compare(k, o)
}

// Scan implement sql.Scanner, allowing a [Key] to
// be directly retrieved from sql backends without
// an intermediary object.
func (k *Key) Scan(scan any) error {
	switch key := scan.(type) {
	case []byte:
		*k = bytes.Clone(key)
	case string:
		*k = []byte(strings.Clone(key))
	default:
		return fmt.Errorf("invalid Key type %T", scan)
	}

	return nil
}
