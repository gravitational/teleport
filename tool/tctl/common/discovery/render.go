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

package discovery

import (
	"strings"
)

func displayIntegrationName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "none (ambient credentials)"
	}
	return name
}

// suggestedOrFallbackLimit returns the suggested limit if available,
// otherwise falls back to 5x the current limit.
func suggestedOrFallbackLimit(suggested, current int) int {
	if suggested > 0 {
		return suggested
	}
	return current * 5
}
