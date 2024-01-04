/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package output

import (
	"encoding/json"
	"strings"
)

// ParseJSONFromModel parses a JSON object from the model output and attempts to sanitize contaminant text
// to avoid triggering self-correction due to some natural language being bundled with the JSON.
// The output type is generic, and thus the structure of the expected JSON varies depending on T.
func ParseJSONFromModel[T any](text string) (T, error) {
	cleaned := strings.TrimSpace(text)
	if strings.Contains(cleaned, "```json") {
		cleaned = strings.Split(cleaned, "```json")[1]
	}
	if strings.Contains(cleaned, "```") {
		cleaned = strings.Split(cleaned, "```")[0]
	}
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)
	var output T
	err := json.Unmarshal([]byte(cleaned), &output)
	if err != nil {
		return output, newInvalidOutputErrorWithParseError(err)
	}

	return output, nil
}
