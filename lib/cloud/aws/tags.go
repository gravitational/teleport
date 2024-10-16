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

package aws

import "strings"

const (
	// TagKeyTeleportCreated defines a tag key that indicates that the cloud
	// resource is created by Teleport.
	TagKeyTeleportCreated = "teleport.dev/created"

	// TagKeyTeleportCluster defines a tag key that specifies the Teleport
	// cluster that created the resource.
	TagKeyTeleportCluster = "teleport.dev/cluster"

	// TagKeyTeleportManaged defines a tag key that indicates that the cloud
	// resource is being managed by Teleport.
	TagKeyTeleportManaged = "teleport.dev/managed"

	// TagValueTrue is the tag value "true" in string format.
	TagValueTrue = "true"
)

// IsTagValueTrue checks whether a tag value is true.
func IsTagValueTrue(value string) bool {
	// Here doing a lenient negative check. Any other value is assumed to be
	// true.
	switch strings.ToLower(value) {
	case "false", "no", "disable", "disabled":
		return false
	default:
		return true
	}
}
