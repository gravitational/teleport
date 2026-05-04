/**
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

import { ResourceLabel } from 'teleport/services/agents';

// DesktopLogin describes a desktop login with metadata about whether it requires a request.
export type DesktopLogin = {
  login: string;
  requiresRequest?: boolean;
};

// Desktop is a remote desktop.
export type Desktop = {
  kind: 'windows_desktop' | 'linux_desktop';
  // OS is the os of this desktop.
  os: 'windows' | 'linux' | 'darwin';
  // Name is name (uuid) of the windows desktop.
  name: string;
  // Addr is the network address the desktop can be reached at.
  addr: string;
  // Labels.
  labels: ResourceLabel[];
  // The list of logins this user can use on this desktop.
  logins: string[];
  // DesktopLoginDetails provides per-login metadata (e.g. whether a login requires an access request).
  desktopLoginDetails?: DesktopLogin[];

  host_id?: string;
  host_addr?: string;
  requiresRequest?: boolean;
  // SupportedFeatureIDs contains ComponentFeatureIDs supported by this Desktop.
  supportedFeatureIds?: number[];
};
