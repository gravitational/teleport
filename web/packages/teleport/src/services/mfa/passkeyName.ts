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

import { resolveAuthenticatorName } from 'design/AuthenticatorIcon';

import { aaguidFromCredential, transportsOf } from './aaguid';

// The longest device name the server accepts, mirroring mfaDeviceNameMaxLen (lib/auth/auth.go).
// The server measures it in bytes rather than characters, so every length check here goes
// through utf8Length.
// The AuthenticatorIcon generator clips vendor names to the same budget; deviceNameLimit
// .test.ts holds it to that.
export const MAX_DEVICE_NAME_BYTES = 30;

// composePasskeyName builds a friendly default nickname for a freshly created credential from the
// authenticator it came from: the vendor name behind its AAGUID, or a generic label derived from the
// transports and attachment when the AAGUID is unknown.
export function composePasskeyName(cred: Credential) {
  const name = resolveAuthenticatorName(
    aaguidFromCredential(cred),
    transportsOf(cred),
    (cred as PublicKeyCredential).authenticatorAttachment ?? ''
  );

  // The generated spec map clips names to 30 characters, which can still exceed the
  // server's 30-byte limit once a name carries multibyte characters.
  return clipUtf8(name, MAX_DEVICE_NAME_BYTES);
}

// The maximum number of attempts uniquePasskeyName will make to find a free name before
// giving up and returning the plain name.
const MAX_ATTEMPTS = 100;

// uniquePasskeyName appends a counter when the suggested name is already registered. Every credential
// from a given authenticator composes to the same name, so without this a second passkey from the same
// device would be rejected by the server's device name uniqueness check.
export function uniquePasskeyName(name: string, taken: string[]) {
  const used = new Set(taken.map(n => n.trim().toLowerCase()));

  if (!used.has(name.trim().toLowerCase())) {
    return name;
  }

  for (let n = 2; n <= MAX_ATTEMPTS; n++) {
    const suffix = ` (${n})`;
    const candidate = `${clipUtf8(name, MAX_DEVICE_NAME_BYTES - suffix.length).trimEnd()}${suffix}`;

    if (!used.has(candidate.toLowerCase())) {
      return candidate;
    }
  }

  // Out of candidates: hand back the plain name and let the user rename it.
  return name;
}

const utf8Length = (s: string) => new TextEncoder().encode(s).length;

// Truncates to at most max UTF-8 bytes without splitting a character.
function clipUtf8(s: string, max: number) {
  if (utf8Length(s) <= max) {
    return s;
  }

  let out = '';
  for (const char of s) {
    if (utf8Length(out + char) > max) {
      break;
    }

    out += char;
  }

  return out;
}
