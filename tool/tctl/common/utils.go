// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package common

import (
	"cmp"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/charlievieth/strcase"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// concreteEnum is a protobuf enum that's backed by an int32 (like regular
// protobuf enums are).
type concreteEnum interface {
	protoreflect.Enum
	~int32
}

// parseProtobufEnum is a [kingpin.Action] that will take the collected value in
// the string (which the associated [kingpin.FlagClause] should probably be set
// to output into) and will attempt to parse it as a protobuf enum value, trying
// to match the string against the given shortcuts first.
func parseProtobufEnum[E concreteEnum](s *string, e *E, shortcuts map[string]E) kingpin.Action {
	return func(*kingpin.ParseContext) error {
		// as a [kingpin.Action], this runs after parsing, so the string value
		// is filled in
		s := *s

		// if the value is a known shortcut
		v, ok := shortcuts[s]
		if ok {
			*e = v
			return nil
		}
		// if the value is a known shortcut but with a different case
		for k, v := range shortcuts {
			if strcase.EqualFold(s, k) {
				*e = v
				return nil
			}
		}

		// if the user wants to use some mode that exists but doesn't have
		// a shortcut
		desc := (*e).Descriptor().Values().ByName(protoreflect.Name(s))
		if desc != nil {
			*e = E(desc.Number())
			return nil
		}
		// in case the user wants to use some mode that isn't even known to this
		// tctl build, by numerical enum value
		num, err := strconv.ParseInt(s, 10, 32)
		if err == nil {
			*e = E(num)
			return nil
		}

		return trace.Errorf(
			"invalid value %q, expected one of: %v",
			s,
			strings.Join(
				slices.SortedFunc(
					maps.Keys(shortcuts),
					func(a, b string) int {
						return cmp.Compare(shortcuts[a], shortcuts[b])
					},
				),
				", ",
			),
		)
	}
}
