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

package servicecfg

import "github.com/gravitational/teleport/api/types"

// JamfConfig is the configuration for the Jamf MDM service.
type JamfConfig struct {
	// Spec is the configuration spec.
	Spec *types.JamfSpecV1
	// ExitOnSync controls whether the service performs a single sync operation
	// before exiting.
	ExitOnSync bool
}

func (j *JamfConfig) Enabled() bool {
	return j != nil && j.Spec != nil && j.Spec.Enabled
}
