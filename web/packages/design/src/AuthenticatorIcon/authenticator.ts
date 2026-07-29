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

import { authenticatorIcons } from './authenticatorIcons';

// The name table is generated into the Go package that embeds it, so the server and the web UI name
// authenticators identically. go:embed cannot reach outside its own package, hence the reach upwards.
import aaguidNames from '../../../../../lib/auth/webauthn/aaguid/aaguids.json';

// Typed loosely on purpose: the generated table has 373 keys, and letting TypeScript infer a literal
// type for every one of them costs check time for no benefit.
const names: Record<string, string> = aaguidNames;

// The all-zero AAGUID is what authenticators report when they decline to identify their
// make and model, so it carries no naming information and is treated as absent.
const ZERO_AAGUID = '00000000-0000-0000-0000-000000000000';

/**
 * Resolves a human-readable name for an authenticator.
 *
 * A known AAGUID resolves to the normalized vendor name from the generated table. Otherwise, the
 * WebAuthn transports and attachment hints are used to pick a generic label. `table` is injectable so
 * callers (and tests) can supply their own map instead of the generated default.
 */
export function resolveAuthenticatorName(
  aaguid: string | undefined,
  transports: string[] = [],
  attachment = '',
  table: Record<string, string> = names
) {
  const name = authenticatorName(aaguid, table);
  if (name) {
    return name;
  }

  const has = (t: string) => transports.includes(t);
  if (attachment === 'platform' || has('internal')) {
    return 'Passkey on this device';
  }

  if (has('hybrid')) {
    return 'Phone passkey';
  }

  if (
    attachment === 'cross-platform' ||
    has('usb') ||
    has('nfc') ||
    has('ble')
  ) {
    return 'Security key';
  }

  return 'Passkey';
}

/**
 * The vendor name behind an AAGUID, or undefined when the authenticator did not identify itself or is
 * not in the generated table. Unlike resolveAuthenticatorName this never falls back to a generic label.
 */
export function authenticatorName(
  aaguid?: string,
  table: Record<string, string> = names
) {
  // hasOwn keeps an AAGUID from reaching Object.prototype: a device reporting "constructor" would
  // otherwise resolve to the Object constructor.
  if (!aaguid || aaguid === ZERO_AAGUID || !Object.hasOwn(table, aaguid)) {
    return undefined;
  }

  return table[aaguid];
}

/** The light and dark logo for an AAGUID, if the vendor supplied one. */
export function authenticatorLogo(aaguid?: string) {
  if (
    !aaguid ||
    aaguid === ZERO_AAGUID ||
    !Object.hasOwn(authenticatorIcons, aaguid)
  ) {
    return undefined;
  }

  return authenticatorIcons[aaguid];
}
