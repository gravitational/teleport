/**
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

import { execFileSync, type SpawnSyncReturns } from 'child_process';

import { tctlBin, teleportConfig } from './env';

const inviteURLRe = /https?:\/\/\S+\/web\/invite\/[0-9a-f]+/;
const inviteURLHostRe = /:\/\/[^/]+/;

export function generateInviteURL(username: string, roles = 'access,editor') {
  const baseURL = process.env.START_URL || '';
  const proxyOrigin = baseURL ? new URL(baseURL).origin : '';

  const out = tctl('users', 'add', username, `--roles=${roles}`);

  const match = out.match(inviteURLRe);
  if (!match) {
    throw new Error(`failed to parse invite URL from tctl output: ${out}`);
  }

  let inviteURL = match[0];
  // tctl may output a placeholder like https://<proxyhost>:3080/... when it can't determine
  // the proxy public address. Replace the host with the known proxy address.
  if (proxyOrigin) {
    inviteURL = inviteURL.replace(
      inviteURLHostRe,
      new URL(proxyOrigin).origin.replace(/^https?/, '')
    );
  }

  return inviteURL;
}

export function deleteUser(username: string) {
  tctl('users', 'rm', username);
}

// Removes a resource (e.g. `role/test-role`) if it exists. Swallows only the
// "not found" error tctl returns when the resource is absent; any other
// failure (auth, network, etc.) is re-thrown so it doesn't get masked.
export function deleteResourceIfExists(resource: string) {
  try {
    tctl('rm', resource);
  } catch (err) {
    if (!isNotFoundError(err)) {
      throw err;
    }
  }
}

function isNotFoundError(err: unknown) {
  if (!err || typeof err !== 'object') {
    return false;
  }
  const stderr = (err as SpawnSyncReturns<string>).stderr ?? '';
  return /not found/i.test(stderr);
}

function tctl(...args: string[]) {
  return execFileSync(tctlBin, [...args, '-c', teleportConfig], {
    encoding: 'utf-8',
    stdio: ['pipe', 'pipe', 'pipe'],
  });
}
