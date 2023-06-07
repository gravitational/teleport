/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { WebauthnAssertionResponse } from '../auth';

export type MfaDevice = {
  id: string;
  name: string;
  description: string;
  registeredDate: Date;
  lastUsedDate: Date;
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
