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

import { MfaAuthenticateChallenge } from 'teleport/services/mfa';

import { Auth2faType } from 'shared/services/types';

export function createMfaOptionsFromAuth2faType(auth2faType: Auth2faType) {
  const mfaOptions: MfaOption[] = [];

  if (auth2faType === 'webauthn' || auth2faType === 'on') {
    mfaOptions.push({ value: 'webauthn', label: 'Passkey or Security Key' });
  }

  if (auth2faType === 'otp' || auth2faType === 'on') {
    mfaOptions.push({ value: 'otp', label: 'Authenticator App' });
  }

  return mfaOptions;
}

export function createMfaOptions(mfaChallenge: MfaAuthenticateChallenge) {
  const mfaOptions: MfaOption[] = [];

  if (mfaChallenge?.webauthnPublicKey) {
    mfaOptions.push({ value: 'webauthn', label: 'Passkey or Security Key' });
  }

  if (mfaChallenge?.totpChallenge) {
    mfaOptions.push({ value: 'otp', label: 'Authenticator App' });
  }

  if (mfaChallenge?.ssoChallenge) {
    mfaOptions.push({
      value: 'sso',
      label:
        mfaChallenge.ssoChallenge.device.displayName ||
        mfaChallenge.ssoChallenge.device.connectorId,
    });
  }

  return mfaOptions;
}

export default createMfaOptions;

export type MfaOption = {
  value: Auth2faType;
  label: string;
};
