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

package internal

import "github.com/gravitational/trace"

const deprecatedRolesURL = "https://goteleport.com/docs/reference/machine-workload-identity/v19-upgrade-guide/"

// CheckDeprecatedRoles returns an error if the removed `roles` field is set.
//
// Services keep the field bound to `yaml:"roles"` rather than deleting it
// because unknown YAML keys are silently discarded, which for a
// privilege-narrowing field would quietly broaden the issued credentials.
//
// TODO(noah): DELETE IN 20.0.0
func CheckDeprecatedRoles(roles []string) error {
	if len(roles) == 0 {
		return nil
	}
	return trace.BadParameter(
		"roles: the roles field is no longer supported and must be removed from your configuration. See %s for further information.",
		deprecatedRolesURL,
	)
}
