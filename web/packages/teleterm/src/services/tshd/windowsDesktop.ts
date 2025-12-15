/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { WindowsDesktop } from 'gen-proto-ts/teleport/lib/teleterm/v1/windows_desktop_pb';

const DEFAULT_RDP_LISTEN_PORT = 3389;

/** Strips the default RDP port from the address since it is unimportant to display. */
export function getWindowsDesktopAddrWithoutDefaultPort(
  desktop: WindowsDesktop
): string {
  const address = desktop.addr;
  const parts = address.split(':');
  if (parts.length === 2 && parts[1] === DEFAULT_RDP_LISTEN_PORT.toString()) {
    return parts[0];
  }
  return address;
}
