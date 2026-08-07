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

import { DeviceType, DeviceUsage, MfaDevice } from './types';

function getType(deviceTypeFromJsonResponse: string): DeviceType {
  if (deviceTypeFromJsonResponse === 'TOTP') {
    return 'totp';
  }
  if (deviceTypeFromJsonResponse === 'SSO') {
    return 'sso';
  }
  return 'webauthn';
}

export type MakeMfaDeviceOptions = {
  isPasswordlessEnabled?: boolean;
};

export default function makeMfaDevice(
  json: any,
  opts: MakeMfaDeviceOptions
): MfaDevice {
  const { id, name, lastUsed, addedAt, residentKey } = json;
  const usage = residentKey ? 'passwordless' : 'mfa';
  const type = getType(json.type);

  return {
    id,
    name,
    description: description(json.type, usage, opts),
    registeredDate: new Date(addedAt),
    lastUsedDate: new Date(lastUsed),
    type,
    usage,
  };
}

function description(
  type: string,
  usage: DeviceUsage,
  opts: MakeMfaDeviceOptions
): string {
  const { isPasswordlessEnabled = false } = opts;
  if (usage === 'passwordless' && isPasswordlessEnabled) {
    return 'Passkey';
  }

  if (type === 'TOTP') {
    return 'Authenticator App';
  }
  if (type === 'U2F' || type === 'WebAuthn') {
    return 'Hardware Key';
  }
  if (type === 'SSO') {
    return 'SSO Provider';
  }
  return 'unknown device';
}
