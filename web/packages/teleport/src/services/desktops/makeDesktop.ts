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

import type { Desktop, WindowsDesktopService } from './types';

export function makeDesktop(json): Desktop {
  const { os, name, addr, host_id, requiresRequest } = json;

  const labels = json.labels || [];
  const logins = json.logins || [];

  return {
    kind: 'windows_desktop',
    os,
    name,
    addr,
    labels,
    host_id,
    logins,
    requiresRequest,
  };
}

export function makeDesktopService(json): WindowsDesktopService {
  const { name, hostname, addr } = json;

  const labels = json.labels || [];

  return {
    kind: 'windows_desktop_service',
    hostname,
    addr,
    labels,
    name,
  };
}
