/*
Copyright 2024 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"strings"

	"github.com/gravitational/trace"
)

// decodeEnum decodes a protobuf enum from a representational value, usually a bool,
// string, or from the actual enum (int32) value. If the value is valid, it is saved
// in the given enum pointer.
func decodeEnum[T ~int32](p *T, val any, representationMap map[any]T, enumMap map[int32]string) error {
	if v, ok := representationMap[val]; ok {
		*p = v
		return nil
	}

	// try parsing as a bool value
	if v, ok := val.(string); ok {
		switch strings.ToLower(v) {
		case "yes", "yeah", "y", "true", "1", "on":
			if v, ok := representationMap[true]; ok {
				*p = v
				return nil
			}
		case "no", "nope", "n", "false", "0", "off":
			if v, ok := representationMap[false]; ok {
				*p = v
				return nil
			}
		}
		return trace.BadParameter("unknown enum value %v", val)
	}

	// parse as enum
	var enumVal int32
	switch v := val.(type) {
	case int:
		enumVal = int32(v)
	case int32:
		enumVal = int32(v)
	case int64:
		enumVal = int32(v)
	case float64:
		enumVal = int32(v)
	case float32:
		enumVal = int32(v)
	default:
		return trace.BadParameter("unknown enum value %v", val)
	}

	if err := checkEnum(enumMap, enumVal); err != nil {
		return trace.BadParameter("unknown enum value %v", val)
	}

	*p = T(enumVal)
	return nil
}

// checkEnum checks if the given enum is valid.
func checkEnum(enumMap map[int32]string, val int32) error {
	if _, ok := enumMap[val]; ok {
		return nil
	}
	return trace.NotFound("enum %v not found in enum map", val)
}
