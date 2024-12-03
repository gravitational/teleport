import { Auth2faType } from 'shared/services';

import { DeviceType, MfaAuthenticateChallenge, SSOChallenge } from './types';

export function getMfaChallengeOptions(mfaChallenge: MfaAuthenticateChallenge) {
  const mfaOptions: MfaOption[] = [];

  if (mfaChallenge?.webauthnPublicKey) {
    mfaOptions.push(webauthnOption);
  }

  if (mfaChallenge?.totpChallenge) {
    mfaOptions.push(totpOption);
  }

  if (mfaChallenge?.ssoChallenge) {
    mfaOptions.push(getSsoOption(mfaChallenge.ssoChallenge));
  }

  return mfaOptions;
}

export function getMfaRegisterOptions(auth2faType: Auth2faType) {
  const mfaOptions: MfaOption[] = [];

  if (auth2faType === 'webauthn' || auth2faType === 'on') {
    mfaOptions.push(webauthnOption);
  }

  if (auth2faType === 'otp' || auth2faType === 'on') {
    mfaOptions.push(totpOption);
  }

  return mfaOptions;
}

export type MfaOption = {
  value: DeviceType;
  label: string;
};

const webauthnOption: MfaOption = {
  value: 'webauthn',
  label: 'Passkey or Security Key',
};

const totpOption: MfaOption = { value: 'totp', label: 'Authenticator App' };

const getSsoOption = (ssoChallenge: SSOChallenge): MfaOption => {
  return {
    value: 'sso',
    label:
      ssoChallenge.device?.displayName ||
      ssoChallenge.device?.connectorId ||
      'SSO',
  };
};
