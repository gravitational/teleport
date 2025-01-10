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

// Adapted from https://developer.mozilla.org/en-US/docs/Web/API/SubtleCrypto/digest#converting_a_digest_to_a_hex_string
export async function Sha256Digest(
  message: string,
  encoder: TextEncoder = new TextEncoder()
): Promise<string> {
  const msgUint8 = encoder.encode(message); // encode as (utf-8) Uint8Array
  const hashBuffer = await crypto.subtle.digest('SHA-256', msgUint8); // hash the message
  const hashArray = Array.from(new Uint8Array(hashBuffer)); // convert buffer to byte array
  const hashHex = hashArray.map(b => b.toString(16).padStart(2, '0')).join(''); // convert bytes to hex string
  return hashHex;
}

export type TshLoginCommand = {
  authType: AuthType;
  clusterId?: string;
  username: string;
  accessRequestId?: string;
};

export function generateTshLoginCommand({
  authType,
  clusterId = '',
  username,
  accessRequestId = '',
}: TshLoginCommand) {
  const { hostname, port } = window.location;
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
