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
	"slices"
	"strings"
)

// Key is the unique identifier for an [Item].
type Key struct {
	s          string
	components []string
	exactKey   bool
	noEnd      bool
}

const (
	// Separator is used as a separator between key parts.
	Separator = '/'
	// SeparatorString is string representation of Separator.
	SeparatorString = string(Separator)
)

// NewKey joins parts into path separated by [Separator],
// makes sure path always starts with [Separator].
func NewKey(components ...string) Key {
	k := internalKey("", components...)
	k.exactKey = k.s != "" && k.s[len(k.s)-1] == Separator
	return k
}

// ExactKey is like [NewKey], except a [Separator] is appended to the
// result path of [Key]. This is to ensure range matching of a path will only
// math child paths and not other paths that have the resulting path
// as a prefix.
func ExactKey(components ...string) Key {
	k := NewKey(append(components, "")...)
	k.exactKey = true
	return k
}

// KeyFromString creates a [Key] from a textual representation
// of the [Key]. No leading or trailing [Separator] are added
// like with [NewKey] or [ExactKey].
func KeyFromString(s string) Key {
	components := strings.Split(s, SeparatorString)
	if components[0] == "" && len(components) > 1 {
		components = components[1:]
	}

	return Key{
		components: components,
		s:          s,
		exactKey:   s == SeparatorString || (s != "" && s[len(s)-1] == Separator),
		noEnd:      s == string(noEnd),
	}
}

func (k Key) IsZero() bool {
	return len(k.components) == 0 && k.s == ""
}

func internalKey(internalPrefix string, components ...string) Key {
	return Key{
		components: components,
		s:          strings.Join(append([]string{internalPrefix}, components...), SeparatorString),
	}
}

// ExactKey appends a [Separator] to the key, if one does not already
// exist. This is to ensure range matching of a path will only
// math child paths and not other paths that have the resulting path
// as a prefix.
func (k Key) ExactKey() Key {
	if k.exactKey {
		return k
	}

	return ExactKey(k.components...)
}

// String returns the textual representation of the key with
// each component concatenated together via the [Separator].
func (k Key) String() string {
	if k.noEnd {
		return string(noEnd)
	}

	return k.s
}

// HasPrefix reports whether the key begins with prefix.
func (k Key) HasPrefix(prefix Key) bool {
	return strings.HasPrefix(k.s, prefix.s)
}

// TrimPrefix returns the key without the provided leading prefix string.
// If the key doesn't start with prefix, it is returned unchanged.
func (k Key) TrimPrefix(prefix Key) Key {
	key := strings.TrimPrefix(k.s, prefix.s)
	if key == "" {
		return Key{}
	}

	return KeyFromString(key)
}

// PrependKey returns a new [Key] that joins p and k
// with the components of p followed by the components from k.
func (k Key) PrependKey(p Key) Key {
	if p.IsZero() {
		return k
	}

	newKey := Key{
		components: append(slices.Clone(p.components), k.components...),
	}
	if strings.HasPrefix(p.s, SeparatorString) {
		newKey.s = strings.Join(append([]string{""}, newKey.components...), SeparatorString)
	} else {
		newKey.s = strings.Join(newKey.components, SeparatorString)
	}

	return newKey
}

// AppendKey returns a new [Key] that joins p and k
// with the components of k followed by the components from p.
func (k Key) AppendKey(p Key) Key {
	if k.IsZero() {
		return p
	}

	newKey := Key{
		components: append(k.components, slices.Clone(p.components)...),
	}
	if strings.HasPrefix(k.s, SeparatorString) {
		newKey.s = strings.Join(append([]string{""}, newKey.components...), SeparatorString)
	} else {
		newKey.s = strings.Join(newKey.components, SeparatorString)
	}

	return newKey
}

// HasSuffix reports whether the key ends with suffix.
func (k Key) HasSuffix(suffix Key) bool {
	return strings.HasSuffix(k.s, suffix.s)
}

// TrimSuffix returns the key without the provided trailing suffix string.
// If the key doesn't end with suffix, it is returned unchanged.
func (k Key) TrimSuffix(suffix Key) Key {
	key := strings.TrimSuffix(k.s, suffix.s)
	if key == "" {
		return Key{}
	}

	return KeyFromString(key)
}

// Components returns the individual components that make up the [Key].
func (k Key) Components() []string {
	return slices.Clone(k.components)
}

// Compare returns an integer comparing two [Key]s lexicographically.
// The result will be 0 if a == b, -1 if a < b, and +1 if a > b.
func (k Key) Compare(o Key) int {
	return strings.Compare(k.s, o.s)
}

// Scan implement sql.Scanner, allowing a [Key] to
// be directly retrieved from sql backends without
// an intermediary object.
func (k *Key) Scan(scan any) error {
	switch key := scan.(type) {
	case []byte:
		if len(key) == 0 {
			return nil
		}
		*k = KeyFromString(string(bytes.Clone(key)))
	case string:
		if key == "" {
			return nil
		}

		*k = KeyFromString(key)
	default:
		return fmt.Errorf("invalid Key type %T", scan)
	}

	return nil
}
