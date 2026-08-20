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

// Removes a user if present, swallowing only tctl's "not found" error so it can
// be used to clean up before a test (and on retries) without failing when the
// user doesn't exist yet.
export function deleteUserIfExists(username: string) {
  try {
    tctl('users', 'rm', username);
  } catch (err) {
    if (!isNotFoundError(err)) {
      throw err;
    }
  }
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
