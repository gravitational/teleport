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

package constants

import "time"

const (
	// MaintenancePath is the version discovery endpoint representing if the
	// target version represents a critical update as defined in
	// https://github.com/gravitational/teleport/blob/master/rfd/0109-cloud-agent-upgrades.md#version-discovery-endpoint
	MaintenancePath = "critical"
	// VersionPath is the version discovery endpoint returning the current
	// target version as defined in
	// https://github.com/gravitational/teleport/blob/master/rfd/0109-cloud-agent-upgrades.md#version-discovery-endpoint
	VersionPath   = "version"
	HTTPTimeout   = 10 * time.Second
	CacheDuration = time.Minute

	// NoVersion is returned by the version endpoint when there is no valid target version.
	// This can be caused by the target version being incompatible with the cluster version.
	NoVersion = "none"
)
