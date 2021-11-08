/**
 * Copyright 2021 Gravitational, Inc.
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

import { Auth2faType, PreferredMfaType } from 'shared/services';

export type MfaOption = {
  value: Auth2faType;
  label: string;
};

export function getMfaOptions(
  mfa: Auth2faType,
  preferredMfa: PreferredMfaType,
  requireMfa = false
) {
  const mfaOptions: MfaOption[] = [];

  if (mfa === 'off' || !mfa) {
    return mfaOptions;
  }

  const mfaEnabled = mfa === 'on' || mfa === 'optional';
  const preferMfaWebauthn = mfaEnabled && preferredMfa === 'webauthn';
  const preferMfaU2f = mfaEnabled && preferredMfa === 'u2f';

  if (mfa === 'webauthn' || preferMfaWebauthn) {
    mfaOptions.push({ value: 'webauthn', label: 'Hardware Key' });
  }

  if (mfa === 'u2f' || preferMfaU2f) {
    mfaOptions.push({ value: 'u2f', label: 'Hardware Key' });
  }

  if (mfa === 'otp' || mfaEnabled) {
    mfaOptions.push({ value: 'otp', label: 'Authenticator App' });
  }

  if (!requireMfa && mfa === 'optional') {
    mfaOptions.push({ value: 'optional', label: 'None' });
  }

  return mfaOptions;
}
