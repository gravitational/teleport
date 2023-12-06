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

package enterprise

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
)

// ProtocolValidation checks if protocol is supported for current build.
func ProtocolValidation(dbProtocol string) error {
	switch dbProtocol {
	case defaults.ProtocolOracle:
		if modules.GetModules().BuildType() != modules.BuildEnterprise {
			return trace.BadParameter("%s database protocol is only available with an enterprise license", dbProtocol)
		}
	}
	return nil
}
