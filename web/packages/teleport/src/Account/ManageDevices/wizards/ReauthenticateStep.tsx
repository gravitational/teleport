/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { OutlineDanger } from 'design/Alert/Alert';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import { RadioGroup } from 'design/RadioGroup';
import React, { useState, FormEvent } from 'react';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { Auth2faType } from 'shared/services';
import createMfaOptions, { MfaOption } from 'shared/utils/createMfaOptions';
import { StepComponentProps, StepHeader } from 'design/StepSlider';

import Box from 'design/Box';

import { Attempt } from 'shared/hooks/useAttemptNext';

import useReAuthenticate from 'teleport/components/ReAuthenticate/useReAuthenticate';
import { MfaDevice } from 'teleport/services/mfa';

export type ReauthenticateStepProps = StepComponentProps & {
  auth2faType: Auth2faType;
  devices: MfaDevice[];
  onAuthenticated(privilegeToken: string): void;
  onClose(): void;
};
export function ReauthenticateStep({
  next,
  refCallback,
  stepIndex,
  flowLength,
  auth2faType,
  devices,
  onClose,
  onAuthenticated: onAuthenticatedProp,
}: ReauthenticateStepProps) {
  const onAuthenticated = (privilegeToken: string) => {
    onAuthenticatedProp(privilegeToken);
    next();
  };
  const { attempt, clearAttempt, submitWithTotp, submitWithWebauthn } =
    useReAuthenticate({
      onAuthenticated,
    });
  const mfaOptions = createReauthOptions(auth2faType, devices);

  const [mfaOption, setMfaOption] = useState<Auth2faType | undefined>(
    mfaOptions[0]?.value
  );
  const [authCode, setAuthCode] = useState('');

  const onAuthCodeChanged = (e: React.ChangeEvent<HTMLInputElement>) => {
    setAuthCode(e.target.value);
  };

  const onReauthenticate = (
    e: FormEvent<HTMLFormElement>,
    validator: Validator
  ) => {
    e.preventDefault();
    if (!validator.validate()) return;
    if (mfaOption === 'webauthn') {
      submitWithWebauthn();
    }
    if (mfaOption === 'otp') {
      submitWithTotp(authCode);
    }
  };

  const errorMessage = getReauthenticationErrorMessage(
    auth2faType,
    mfaOptions.length,
    attempt
  );

  return (
    <div ref={refCallback} data-testid="reauthenticate-step">
      <Box mb={4}>
        <StepHeader
          stepIndex={stepIndex}
          flowLength={flowLength}
          title="Verify Identity"
        />
      </Box>
      {errorMessage && <OutlineDanger>{errorMessage}</OutlineDanger>}
      {mfaOption && 'Multi-factor type'}
      <Validation>
        {({ validator }) => (
          <form onSubmit={e => onReauthenticate(e, validator)}>
            <RadioGroup
              name="mfaOption"
              options={mfaOptions}
              value={mfaOption}
              autoFocus
              flexDirection="row"
              gap={3}
              mb={4}
              onChange={o => {
                setMfaOption(o as Auth2faType);
                clearAttempt();
              }}
            />
            {mfaOption === 'otp' && (
              <FieldInput
                label="Authenticator Code"
                rule={requiredField('Authenticator code is required')}
                inputMode="numeric"
                autoComplete="one-time-code"
                value={authCode}
                placeholder="123 456"
                onChange={onAuthCodeChanged}
                readonly={attempt.status === 'processing'}
              />
            )}
            <Flex gap={2}>
              {mfaOption && (
                <ButtonPrimary type="submit" block={true} size="large">
                  Verify my identity
                </ButtonPrimary>
              )}
              <ButtonSecondary
                type="button"
                block={true}
                onClick={onClose}
                size="large"
              >
                Cancel
              </ButtonSecondary>
            </Flex>
          </form>
        )}
      </Validation>
    </div>
  );
}
function getReauthenticationErrorMessage(
  auth2faType: Auth2faType,
  numMfaOptions: number,
  attempt: Attempt
): string {
  if (numMfaOptions === 0) {
    switch (auth2faType) {
      case 'on':
        return (
          "Identity verification is required, but you don't have any" +
          'passkeys or MFA methods registered. This may mean that the' +
          'server configuration has changed. Please contact your ' +
          'administrator.'
        );
      case 'otp':
        return (
          'Identity verification using authenticator app is required, but ' +
          "you don't have any authenticator apps registered. This may mean " +
          'that the server configuration has changed. Please contact your ' +
          'administrator.'
        );
      case 'webauthn':
        return (
          'Identity verification using a passkey or security key is required, but ' +
          "you don't have any such devices registered. This may mean " +
          'that the server configuration has changed. Please contact your ' +
          'administrator.'
        );
      case 'optional':
      case 'off':
        // This error message is not useful, but this condition should never
        // happen, and if it does, it means something is broken, and we don't
        // have a clue anyway.
        return 'Unable to verify identity';
      default:
        auth2faType satisfies never;
    }
  }

  if (attempt.status === 'failed') {
    // This message relies on the status message produced by the auth server in
    // lib/auth/Server.checkOTP function. Please keep these in sync.
    if (attempt.statusText === 'invalid totp token') {
      return 'Invalid authenticator code';
    } else {
      return attempt.statusText;
    }
  }
}

export function createReauthOptions(
  auth2faType: Auth2faType,
  devices: MfaDevice[]
): MfaOption[] {
  return createMfaOptions({ auth2faType, required: true }).filter(
    ({ value }) => {
      const deviceType = value === 'otp' ? 'totp' : value;
      return devices.some(({ type }) => type === deviceType);
    }
  );
}
