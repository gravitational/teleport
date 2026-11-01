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

package agent

const (
	// windowsTeleportServiceName is the name of the Windows service (SCM) running the Teleport agent.
	windowsTeleportServiceName = "Teleport"
	// UpdaterServiceName is the name of the Windows service (SCM) running the updater loop.
	UpdaterServiceName = "TeleportUpdater"
	// EventSourceName is the Windows Event Log source used by the updater service.
	EventSourceName = "teleport-update"
)

// teleportServiceName returns the name of the Windows service running the Teleport agent.
// Namespaced installations are suffixed with the namespace name.
func (ns *Namespace) teleportServiceName() string {
	if ns.name == "" {
		return windowsTeleportServiceName
	}
	return windowsTeleportServiceName + "_" + ns.name
}
