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

import React, { useState } from 'react';
import { Box, ButtonPrimary, ButtonText, Text } from 'design';
import { Danger } from 'design/Alert';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import {
  requiredConfirmedPassword,
  requiredPassword,
} from 'shared/components/Validation/rules';
import { useRefAutoFocus } from 'shared/hooks';

import { OnboardCard } from 'design/Onboard/OnboardCard';

import { SliderProps, UseTokenState } from './types';

export function NewPassword(props: Props) {
  const {
    submitAttempt,
    resetToken,
    isPasswordlessEnabled,
    onSubmit,
    auth2faType,
    primaryAuthType,
    password,
    updatePassword,
    changeFlow,
    next,
    refCallback,
    hasTransitionEnded,
  } = props;
  const [passwordConfirmed, setPasswordConfirmed] = useState('');
  const mfaEnabled = auth2faType !== 'off';

  function handleOnSubmit(
    e: React.MouseEvent<HTMLButtonElement>,
    validator: Validator
  ) {
    e.preventDefault(); // prevent form submit default

    if (!validator.validate()) {
      return;
    }

    if (mfaEnabled) {
      next();
      return;
    }

    onSubmit(password);
  }

  const passwordInputRef = useRefAutoFocus<HTMLInputElement>({
    shouldFocus: hasTransitionEnded,
  });

  function switchToPasswordlessFlow(e, applyNextAnimation = false) {
    e.preventDefault();
    changeFlow({ flow: 'passwordless', applyNextAnimation });
  }

  return (
    <Validation>
      {({ validator }) => (
        <OnboardCard ref={refCallback} data-testid="password">
          {mfaEnabled && <Text color="text.slightlyMuted">Step 1 of 2</Text>}
          <Text typography="h4" bold color="text.main" mb={3}>
            Set a Password
          </Text>
          {submitAttempt.status === 'failed' && (
            <Danger children={submitAttempt.statusText} />
          )}
          <FieldInput
            label="Username"
            value={resetToken.user}
            onChange={() => null}
            readonly
          />
          <FieldInput
            rule={requiredPassword}
            ref={passwordInputRef}
            autoComplete="off"
            label="Password"
            value={password}
            onChange={e => updatePassword(e.target.value)}
            type="password"
            placeholder="Password"
          />
          <FieldInput
            rule={requiredConfirmedPassword(password)}
            autoComplete="off"
            label="Confirm Password"
            value={passwordConfirmed}
            onChange={e => setPasswordConfirmed(e.target.value)}
            type="password"
            placeholder="Confirm Password"
          />
          <ButtonPrimary
            width="100%"
            mt={3}
            size="large"
            onClick={e => handleOnSubmit(e, validator)}
            disabled={submitAttempt.status === 'processing'}
          >
            {mfaEnabled ? 'Next' : 'Submit'}
          </ButtonPrimary>
          {primaryAuthType !== 'passwordless' && isPasswordlessEnabled && (
            <Box mt={3} textAlign="center">
              <ButtonText
                onClick={e => switchToPasswordlessFlow(e)}
                disabled={submitAttempt.status === 'processing'}
              >
                Go Passwordless
              </ButtonText>
            </Box>
          )}
          {primaryAuthType === 'passwordless' && (
            <Box mt={3} textAlign="center">
              <ButtonText
                onClick={e => switchToPasswordlessFlow(e, true)}
                disabled={submitAttempt.status === 'processing'}
              >
                Back
              </ButtonText>
            </Box>
          )}
        </OnboardCard>
      )}
    </Validation>
  );
}

type Props = UseTokenState &
  SliderProps & {
    password: string;
    updatePassword(pwd: string): void;
  };
