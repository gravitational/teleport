// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package llm

import (
	"cmp"
	"strings"

	"github.com/gravitational/teleport/api/types"
)

// convertModelName takes a set model name and do the conversion based on the
// mapping rules.
func convertModelName(mappings []*types.LLM_Model, fallbackModelName string, reqModel string) (string, bool) {
	if len(mappings) == 0 && reqModel != "" {
		return reqModel, true
	}

	for _, m := range mappings {
		if strings.EqualFold(strings.TrimSpace(reqModel), m.Name) {
			return cmp.Or(m.ProviderName, m.Name), true
		}
	}

	for _, m := range mappings {
		if m.Name == fallbackModelName {
			return cmp.Or(m.ProviderName, m.Name), true
		}
	}

	return "", false
}
