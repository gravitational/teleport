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

import {
  authenticatorSpecs,
  type AuthenticatorSpec,
} from './authenticatorSpecs';

// The all-zero AAGUID is what authenticators report when they decline to identify their
// make and model, so it carries no naming information and is treated as absent.
const ZERO_AAGUID = '00000000-0000-0000-0000-000000000000';

/**
 * Resolves a human-readable name for an authenticator.
 *
 * A known AAGUID resolves to the normalized vendor name from the spec map. Otherwise, the
 * WebAuthn transports and attachment hints are used to pick a generic label. `specs` is
 * injectable so callers (and tests) can supply their own map instead of the generated default.
 */
export function resolveAuthenticatorName(
  aaguid: string | undefined,
  transports: string[] = [],
  attachment = '',
  specs: Record<string, AuthenticatorSpec> = authenticatorSpecs
) {
  const spec = authenticatorSpec(aaguid, specs);
  if (spec) {
    return spec.name;
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

export function authenticatorSpec(
  aaguid?: string,
  specs: Record<string, AuthenticatorSpec> = authenticatorSpecs
): AuthenticatorSpec | undefined {
  if (!aaguid || aaguid === ZERO_AAGUID || !Object.hasOwn(specs, aaguid)) {
    return undefined;
  }

  return specs[aaguid];
}
