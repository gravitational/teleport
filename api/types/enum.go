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

package types

import (
	"encoding/json"

	"github.com/gravitational/trace"
)

// getReciprocalMaps takes an initial map of two comparable types and returns it back
// along with its reciprocal map. Useful for generating consistent between enum values
// and names.
func getReciprocalMaps[K comparable, V comparable](initial map[K]V) (map[K]V, map[V]K) {
	reciprocal := make(map[V]K, len(initial))
	for k, v := range initial {
		reciprocal[v] = k
	}

	return initial, reciprocal
}

// unmarshalEnumJSON is a generic function for JSON unmarshaling protobuf enum types using string mappings
// or their original int32 value
func unmarshalEnumJSON[T ~int32](valToName map[T]string, nameToVal map[string]T, data []byte, dst *T) error {
	var anyVal any
	if err := json.Unmarshal(data, &anyVal); err != nil {
		return trace.Wrap(err)
	}

	var castVal T
	switch val := anyVal.(type) {
	case string:
		enumVal, ok := nameToVal[val]
		if !ok {
			return trace.Errorf("invalid value: %q", val)
		}
		*dst = enumVal
		return nil
	case uint:
		castVal = T(val)
	case uint32:
		castVal = T(val)
	case uint64:
		castVal = T(val)
	case int:
		castVal = T(val)
	case int32:
		castVal = T(val)
	case int64:
		castVal = T(val)
	case float64:
		castVal = T(val)
	case float32:
		castVal = T(val)
	default:
		return trace.BadParameter("unexpected type %T", val)
	}

	if _, ok := valToName[castVal]; ok {
		return trace.BadParameter("invalid value: %d", castVal)
	}

	*dst = castVal
	return nil
}
