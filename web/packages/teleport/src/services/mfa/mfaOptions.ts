import { Auth2faType } from 'shared/services';
import { DeviceType, MfaAuthenticateChallenge } from './types';

export type MfaOption = {
  value: DeviceType;
  label: string;
};

export function getMfaChallengeOptions(mfaChallenge: MfaAuthenticateChallenge) {
  const mfaOptions: MfaOption[] = [];

  if (mfaChallenge?.webauthnPublicKey) {
    mfaOptions.push({ value: 'webauthn', label: 'Passkey or Security Key' });
  }

  if (mfaChallenge?.totpChallenge) {
    mfaOptions.push({ value: 'totp', label: 'Authenticator App' });
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

export function getMfaRegisterOptions(auth2faType: Auth2faType) {
  const mfaOptions: MfaOption[] = [];

  if (auth2faType === 'webauthn' || auth2faType === 'on') {
    mfaOptions.push({ value: 'webauthn', label: 'Passkey or Security Key' });
  }

  if (auth2faType === 'otp' || auth2faType === 'on') {
    mfaOptions.push({ value: 'totp', label: 'Authenticator App' });
  }

  return mfaOptions;
}
