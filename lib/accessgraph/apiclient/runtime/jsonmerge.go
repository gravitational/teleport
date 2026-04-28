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

package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// JSONMerge merges patch into data using the object-merge behavior expected by
// the generated union helpers.
func JSONMerge(data, patch json.RawMessage) (json.RawMessage, error) {
	if data == nil {
		data = []byte(`{}`)
	}
	if patch == nil {
		patch = []byte(`{}`)
	}

	var dataValue any
	if err := unmarshalJSON(data, &dataValue); err != nil {
		return nil, fmt.Errorf("error in data JSON: %w", err)
	}

	var patchValue any
	if err := unmarshalJSON(patch, &patchValue); err != nil {
		return nil, fmt.Errorf("error in patch JSON: %w", err)
	}

	merged, err := json.Marshal(mergeJSON(dataValue, patchValue))
	if err != nil {
		return nil, fmt.Errorf("error writing merged JSON: %w", err)
	}
	return merged, nil
}

func unmarshalJSON(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder.Decode(value)
}

func mergeJSON(data, patch any) any {
	patchObject, ok := patch.(map[string]any)
	if !ok {
		return data
	}

	switch dataObject := data.(type) {
	case map[string]any:
		merged := make(map[string]any, len(dataObject)+len(patchObject))
		for key, value := range dataObject {
			merged[key] = mergeJSONValue(value, patchObject, key)
		}
		for key, value := range patchObject {
			if _, ok := dataObject[key]; !ok {
				merged[key] = value
			}
		}
		return merged
	case []any:
		merged := make([]any, len(dataObject))
		for i, value := range dataObject {
			merged[i] = mergeJSONValue(value, patchObject, strconv.Itoa(i))
		}
		return merged
	default:
		return data
	}
}

func mergeJSONValue(value any, patchObject map[string]any, key string) any {
	patchValue, ok := patchObject[key]
	if !ok {
		return value
	}

	if _, valueIsObject := value.(map[string]any); valueIsObject {
		if _, patchIsObject := patchValue.(map[string]any); patchIsObject {
			return mergeJSON(value, patchValue)
		}
		return value
	}

	if _, valueIsArray := value.([]any); valueIsArray {
		if _, patchIsObject := patchValue.(map[string]any); patchIsObject {
			return mergeJSON(value, patchValue)
		}
	}

	return patchValue
}
