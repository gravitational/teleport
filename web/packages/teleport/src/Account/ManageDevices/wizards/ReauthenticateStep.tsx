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
import React, { useState, FormEvent, useEffect } from 'react';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { StepComponentProps, StepHeader } from 'design/StepSlider';

import Box from 'design/Box';

import { Attempt } from 'shared/hooks/useAttemptNext';

import { DeviceType } from 'teleport/services/mfa';
import { MfaOption } from 'teleport/services/mfa';

export type ReauthenticateStepProps = StepComponentProps & {
  reauthAttempt: Attempt;
  clearReauthAttempt(): void;
  mfaChallengeOptions: MfaOption[];
  submitWithMfa(mfaType?: DeviceType, otpCode?: string): Promise<void>;
  onClose(): void;
};

export function ReauthenticateStep({
  next,
  refCallback,
  stepIndex,
  flowLength,
  mfaChallengeOptions,
  reauthAttempt,
  clearReauthAttempt,
  submitWithMfa,
  onClose,
}: ReauthenticateStepProps) {
  const [otpCode, setOtpCode] = useState('');
  const [mfaOption, setMfaOption] = useState(mfaChallengeOptions[0].value);

  const onOtpCodeChanged = (e: React.ChangeEvent<HTMLInputElement>) => {
    setOtpCode(e.target.value);
  };

  const onReauthenticate = (
    e: FormEvent<HTMLFormElement>,
    validator: Validator
  ) => {
    e.preventDefault();
    if (!validator.validate()) return;
    submitWithMfa(mfaOption, otpCode).then(next);
  };

  const errorMessage = getReauthenticationErrorMessage(reauthAttempt);

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
      {mfaOption && <Box mb={2}>Multi-factor type</Box>}
      <Validation>
        {({ validator }) => (
          <form onSubmit={e => onReauthenticate(e, validator)}>
            <RadioGroup
              name="mfaOption"
              options={mfaChallengeOptions}
              value={mfaOption}
              autoFocus
              flexDirection="row"
              gap={3}
              mb={4}
              onChange={o => {
                setMfaOption(o as DeviceType);
                clearReauthAttempt();
              }}
            />
            {mfaOption === 'totp' && (
              <FieldInput
                label="Authenticator Code"
                rule={requiredField('Authenticator code is required')}
                inputMode="numeric"
                autoComplete="one-time-code"
                value={otpCode}
                placeholder="123 456"
                onChange={onOtpCodeChanged}
                readonly={reauthAttempt.status === 'processing'}
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

function getReauthenticationErrorMessage(attempt: Attempt): string {
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
