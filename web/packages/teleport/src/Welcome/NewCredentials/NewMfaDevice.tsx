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

import {
  SliderProps,
  UseTokenState,
} from 'teleport/Welcome/NewCredentials/types';

import { NewMfaDeviceForm } from '../../components/NewMfaDeviceForm';

export function NewMfaDevice({
  resetToken,
  submitAttempt,
  credential,
  clearSubmitAttempt,
  auth2faType,
  createNewWebAuthnDevice,
  onSubmitWithWebauthn,
  onSubmit,
  password,
  prev,
  refCallback,
  hasTransitionEnded,
  stepIndex,
  flowLength,
}: NewMfaDeviceProps) {
  function onFormSubmitWithWebAuthn(deviceName: string) {
    onSubmitWithWebauthn(password, deviceName);
  }
  function onFormSubmit(otpCode: string, deviceName: string) {
    onSubmit(password, otpCode, deviceName);
  }
  function formCreateNewWebAuthnDevice() {
    createNewWebAuthnDevice('mfa');
  }
  return (
    <div ref={refCallback}>
      <NewMfaDeviceForm
        title="Set up Multi-Factor Authentication"
        submitButtonText="Submit"
        submitAttempt={submitAttempt}
        clearSubmitAttempt={clearSubmitAttempt}
        qrCode={resetToken.qrCode}
        auth2faType={auth2faType}
        credential={credential}
        createNewWebAuthnDevice={formCreateNewWebAuthnDevice}
        onSubmitWithWebAuthn={onFormSubmitWithWebAuthn}
        onSubmit={onFormSubmit}
        prev={prev}
        shouldFocus={hasTransitionEnded}
        stepIndex={stepIndex}
        flowLength={flowLength}
      />
    </div>
  );
}

export type NewMfaDeviceProps = UseTokenState &
  SliderProps & {
    password: string;
    updatePassword(pwd: string): void;
  };
