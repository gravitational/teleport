import { Auth2faType } from 'shared/services';
import { Option } from 'shared/components/Select';

export type MfaOption = Option<Auth2faType>;

export function getMfaOptions(auth2faType: Auth2faType) {
  const mfaOptions: MfaOption[] = [];
  const mfaEnabled = auth2faType === 'on' || auth2faType === 'optional';

  if (auth2faType === 'optional' || auth2faType === 'off') {
    mfaOptions.push({ value: 'optional', label: 'None' });
  }

  if (auth2faType === 'u2f' || mfaEnabled) {
    mfaOptions.push({ value: 'u2f', label: 'Hardware Key' });
  }

  if (auth2faType === 'otp' || mfaEnabled) {
    mfaOptions.push({ value: 'otp', label: 'Authenticator App' });
  }

  return mfaOptions;
}
