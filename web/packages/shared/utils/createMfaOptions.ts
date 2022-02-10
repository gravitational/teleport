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

import { Auth2faType, PreferredMfaType } from 'shared/services/types';

export default function createMfaOptions(opts: Options) {
  const { auth2faType, preferredType, required = false } = opts;
  const mfaOptions: MfaOption[] = [];

  if (auth2faType === 'off' || !auth2faType) {
    return mfaOptions;
  }

  const enabled = auth2faType === 'on' || auth2faType === 'optional';
  const preferWebauthn = enabled && preferredType === 'webauthn';
  const preferU2f = enabled && preferredType === 'u2f';

  if (auth2faType === 'webauthn' || preferWebauthn) {
    mfaOptions.push({ value: 'webauthn', label: 'Hardware Key' });
  }

  if (auth2faType === 'u2f' || preferU2f) {
    mfaOptions.push({ value: 'u2f', label: 'Hardware Key' });
  }

  if (auth2faType === 'otp' || enabled) {
    mfaOptions.push({ value: 'otp', label: 'Authenticator App' });
  }

  if (!required && auth2faType === 'optional') {
    mfaOptions.push({ value: 'optional', label: 'None' });
  }

  return mfaOptions;
}

export type MfaOption = {
  value: Auth2faType;
  label: string;
};

type Options = {
  auth2faType: Auth2faType;
  preferredType: PreferredMfaType;
  required?: boolean;
};
