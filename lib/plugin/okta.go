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

package plugin

import (
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

func OktaParseTimeBetweenImports(syncSettings *types.PluginOktaSyncSettings) (time.Duration, error) {
	if syncSettings == nil {
		return 0, nil
	}
	raw := syncSettings.TimeBetweenImports
	if raw == "" {
		return 0, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, trace.BadParameter("time_between_imports is not valid: %s", err)
	}
	if parsed < 0 {
		return 0, trace.BadParameter("time_between_imports %q cannot be a negative value", raw)
	}
	return parsed, nil
}
