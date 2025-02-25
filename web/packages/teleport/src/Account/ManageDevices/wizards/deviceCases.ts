/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { MfaDevice } from 'teleport/services/mfa';

/** A dummy MFA device for test purposes. */
export const dummyHardwareDevice: MfaDevice = {
  id: '2',
  description: 'Hardware Key',
  name: 'solokey',
  registeredDate: new Date(1623722252),
  lastUsedDate: new Date(1623981452),
  type: 'webauthn',
  usage: 'mfa',
};

/** A dummy MFA device for test purposes. */
export const dummyPasskey: MfaDevice = {
  id: '3',
  description: 'Hardware Key',
  name: 'TouchID',
  registeredDate: new Date(1623722252),
  lastUsedDate: new Date(1623981452),
  type: 'webauthn',
  usage: 'passwordless',
};

/** A dummy MFA device for test purposes. */
export const dummyAuthenticatorApp: MfaDevice = {
  id: '4',
  description: 'Authenticator App',
  name: 'iphone 12',
  registeredDate: new Date(1623722252),
  lastUsedDate: new Date(1623981452),
  type: 'totp',
  usage: 'mfa',
};

/**
 * Repeats devices twice to make sure we support multiple devices of the same
 * type and purpose.
 */
function twice(arr) {
  return [...arr, ...arr];
}

/** Dummy devices for testing purposes. */
export const deviceCases: Record<string, MfaDevice[]> = {
  all: twice([dummyAuthenticatorApp, dummyHardwareDevice, dummyPasskey]),
  authApps: twice([dummyAuthenticatorApp]),
  mfaDevices: twice([dummyHardwareDevice]),
  passkeys: twice([dummyPasskey]),
  none: [],
};
