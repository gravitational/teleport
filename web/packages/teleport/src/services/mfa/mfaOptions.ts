/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { Auth2faType } from 'shared/services';

import { DeviceType, MfaAuthenticateChallenge, SsoChallenge } from './types';

// returns mfa challenge options in order of preferences: WebAuthn > SSO > TOTP.
export function getMfaChallengeOptions(mfaChallenge: MfaAuthenticateChallenge) {
  const mfaOptions: MfaOption[] = [];

  if (mfaChallenge?.webauthnPublicKey) {
    mfaOptions.push(MFA_OPTION_WEBAUTHN);
  }

  if (mfaChallenge?.ssoChallenge) {
    mfaOptions.push(getSsoMfaOption(mfaChallenge.ssoChallenge));
  }

  if (mfaChallenge?.totpChallenge) {
    mfaOptions.push(MFA_OPTION_TOTP);
  }

  return mfaOptions;
}

export function getMfaRegisterOptions(auth2faType: Auth2faType) {
  const mfaOptions: MfaOption[] = [];

  if (auth2faType === 'webauthn' || auth2faType === 'on') {
    mfaOptions.push(MFA_OPTION_WEBAUTHN);
  }

  if (auth2faType === 'otp' || auth2faType === 'on') {
    mfaOptions.push(MFA_OPTION_TOTP);
  }

  return mfaOptions;
}

export type MfaOption = {
  value: DeviceType;
  label: string;
};

export const MFA_OPTION_WEBAUTHN: MfaOption = {
  value: 'webauthn',
  label: 'Passkey or Security Key',
};

export const MFA_OPTION_TOTP: MfaOption = {
  value: 'totp',
  label: 'Authenticator App',
};

// SSO MFA option used in tests.
export const MFA_OPTION_SSO_DEFAULT: MfaOption = {
  value: 'sso',
  label: 'SSO',
};

const getSsoMfaOption = (ssoChallenge: SsoChallenge): MfaOption => {
  return {
    value: 'sso',
    label:
      ssoChallenge?.device?.displayName ||
      ssoChallenge?.device?.connectorId ||
      'SSO',
  };
};
