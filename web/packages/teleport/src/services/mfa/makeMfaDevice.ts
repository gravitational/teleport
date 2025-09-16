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

import { MfaDevice } from './types';

export default function makeMfaDevice(json): MfaDevice {
  const { id, name, lastUsed, addedAt, residentKey } = json;

  let description = '';
  if (json.type === 'TOTP') {
    description = 'Authenticator App';
  } else if (json.type === 'U2F' || json.type === 'WebAuthn') {
    description = 'Hardware Key';
  } else {
    description = 'unknown device';
  }

  const type = json.type === 'TOTP' ? 'totp' : 'webauthn';
  const usage = residentKey ? 'passwordless' : 'mfa';

  return {
    id,
    name,
    description,
    registeredDate: new Date(addedAt),
    lastUsedDate: new Date(lastUsed),
    type,
    usage,
  };
}
