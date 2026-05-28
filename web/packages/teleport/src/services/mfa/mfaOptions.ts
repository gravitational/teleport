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

// MFA option seen in challenges, but not registration.
const getSsoMfaOption = (ssoChallenge: SsoChallenge): MfaOption => {
  return {
    value: 'sso',
    label:
      ssoChallenge?.device?.displayName ||
      ssoChallenge?.device?.connectorId ||
      'SSO',
  };
};

// Returns the MFA options available for the given auth2faType.
// 
// 'none' is included only when auth2faType is 'optional'. Callers that
// don't want to offer 'none' (e.g. login) should filter it out.
export function getMfaRegisterOptions(
  auth2faType: Auth2faType
): MfaRegisterOption[] {
  const mfaOptions: MfaRegisterOption[] = [];

  if (auth2faType === 'off' || !auth2faType) {
    return mfaOptions;
  }

  const mfaEnabled = auth2faType === 'on' || auth2faType === 'optional';

  if (auth2faType === 'webauthn' || mfaEnabled) {
    mfaOptions.push(MFA_OPTION_WEBAUTHN);
  }

  if (auth2faType === 'otp' || mfaEnabled) {
    mfaOptions.push(MFA_OPTION_TOTP);
  }

  if (auth2faType === 'optional') {
    mfaOptions.push(MFA_OPTION_NONE);
  }

  return mfaOptions;
}

// Registration adds a 'none' option to skip MFA setup when auth2faType is 'optional'.
// Callers that don't want to offer 'none' (e.g., login) should filter it out.
export type MfaRegisterOption = {
  value: DeviceType | 'none';
  label: string;
};

// MFA option can be seen during registration to represent "skip mfa registration".
export const MFA_OPTION_NONE: MfaRegisterOption = {
  value: 'none',
  label: 'None',
};
