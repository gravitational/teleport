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

import { WebauthnAssertionResponse } from '../auth';

export type MfaDevice = {
  id: string;
  name: string;
  description: string;
  registeredDate: Date;
  lastUsedDate: Date;
  residentKey: boolean;
};

export type AddNewTotpDeviceRequest = {
  tokenId: string;
  deviceName: string;
  secondFactorToken: string;
};

export type AddNewHardwareDeviceRequest = {
  tokenId: string;
  deviceName: string;
  deviceUsage?: DeviceUsage;
};

export type DeviceType = 'totp' | 'webauthn';

// DeviceUsage is the intended usage of the device (MFA, Passwordless, etc).
export type DeviceUsage = 'passwordless' | 'mfa';

// MfaAuthnResponse is a response to a MFA device challenge.
export type MfaAuthnResponse =
  | { totp_code: string }
  | { webauthn_response: WebauthnAssertionResponse };
