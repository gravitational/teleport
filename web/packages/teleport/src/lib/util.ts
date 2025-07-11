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

import type { AuthType } from 'teleport/services/user';

export const openNewTab = (url: string) => {
  const element = document.createElement('a');
  element.setAttribute('href', `${url}`);
  // works in ie11
  element.setAttribute('target', `_blank`);
  element.style.display = 'none';
  document.body.appendChild(element);
  element.click();
  document.body.removeChild(element);
};

export function generateTshLoginCommand({
  authType,
  clusterId = '',
  username,
  accessRequestId = '',
  windowLocation = window.location,
}: {
  authType: AuthType;
  clusterId?: string;
  username: string;
  accessRequestId?: string;
  /** Allows overwriting `window.location` in tests. */
  windowLocation?: {
    hostname: string;
    /** When empty, the default HTTPS port (443) is used. */
    port?: string;
  };
}) {
  const { hostname, port } = windowLocation;
  const host = `${hostname}:${port || '443'}`;
  const requestId = accessRequestId ? ` --request-id=${accessRequestId}` : '';

  switch (authType) {
    case 'sso':
      return `tsh login --proxy=${host} ${clusterId}${requestId}`.trim();
    case 'local':
      return `tsh login --proxy=${host} --auth=${authType} --user=${username} ${clusterId}${requestId}`.trim();
    case 'passwordless':
      return `tsh login --proxy=${host} --auth=${authType} --user=${username} ${clusterId}${requestId}`.trim();
    default:
      throw new Error(`auth type ${authType} is not supported`);
  }
}

/**
 * arrayStrDiff returns an array of strings that
 * belong in stringsA but not in stringsB.
 */
export function arrayStrDiff(stringsA: string[], stringsB: string[]) {
  if (!stringsA || !stringsB) {
    return [];
  }

  return stringsA.filter(l => !stringsB.includes(l));
}

// compareByString is a sort compare function that
// compares by string.
export function compareByString(a: string, b: string) {
  if (a < b) {
    return -1;
  }
  if (a > b) {
    return 1;
  }
  return 0;
}
