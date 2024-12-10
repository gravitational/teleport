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

import { Auth2faType, PreferredMfaType } from 'shared/services/types';

// Deprecated: use getMfaRegisterOptions or getMfaChallengeOptions instead.
// TODO(Joerger): Delete once no longer used.
export default function createMfaOptions(opts: Options) {
  const { auth2faType, required = false } = opts;
  const mfaOptions: MfaOption[] = [];

  if (auth2faType === 'off' || !auth2faType) {
    return mfaOptions;
  }

  const mfaEnabled = auth2faType === 'on' || auth2faType === 'optional';

  if (auth2faType === 'webauthn' || mfaEnabled) {
    mfaOptions.push({ value: 'webauthn', label: 'Passkey or Security Key' });
  }

  if (auth2faType === 'otp' || mfaEnabled) {
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
  // TODO(lisa) remove preferredType, does nothing atm
  preferredType?: PreferredMfaType;
  required?: boolean;
};
