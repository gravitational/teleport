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

import { Box, ButtonPrimary, ButtonText, H2 } from 'design';
import { Danger } from 'design/Alert';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { useRefAutoFocus } from 'shared/hooks';

import { OnboardCard } from 'teleport/components/Onboard';
import { PasskeyBlurb } from 'teleport/components/Passkeys/PasskeyBlurb';

import { SliderProps, UseTokenState } from './types';

export function NewPasswordlessDevice(props: UseTokenState & SliderProps) {
  const {
    submitAttempt,
    credential,
    createNewWebAuthnDevice,
    onSubmitWithWebauthn,
    primaryAuthType,
    isPasswordlessEnabled,
    changeFlow,
    refCallback,
    hasTransitionEnded,
    clearSubmitAttempt,
    resetToken,
  } = props;
  const [deviceName, setDeviceName] = useState('passwordless-device');

  const deviceNameInputRef = useRefAutoFocus<HTMLInputElement>({
    shouldFocus: hasTransitionEnded,
  });

  function handleOnSubmit(
    e: React.MouseEvent<HTMLButtonElement>,
    validator: Validator
  ) {
    e.preventDefault();

    if (!validator.validate()) {
      return;
    }

    if (!credential) {
      createNewWebAuthnDevice('passwordless');
    } else {
      onSubmitWithWebauthn('', deviceName);
    }
  }

  function switchToLocalFlow(e, applyNextAnimation = false) {
    e.preventDefault();
    clearSubmitAttempt();
    changeFlow({ flow: 'local', applyNextAnimation });
  }

  return (
    <Validation>
      {({ validator }) => (
        <OnboardCard ref={refCallback} data-testid="passwordless">
          <H2 mb={3}>Set up Passwordless Authentication</H2>
          {submitAttempt.status === 'failed' && (
            <Danger children={submitAttempt.statusText} />
          )}
          Setting up account for: {resetToken.user}
          <Box mb={3}>
            <PasskeyBlurb />
          </Box>
          {!!credential && (
            <FieldInput
              rule={requiredField('Passkey nickname is required')}
              label="Passkey Nickname"
              placeholder="Name"
              width="100%"
              ref={deviceNameInputRef}
              value={deviceName}
              type="text"
              onChange={e => setDeviceName(e.target.value)}
              readonly={submitAttempt.status === 'processing'}
            />
          )}
          <ButtonPrimary
            width="100%"
            size="large"
            onClick={e => handleOnSubmit(e, validator)}
            disabled={submitAttempt.status === 'processing'}
          >
            {credential ? 'Submit' : 'Create a Passkey'}
          </ButtonPrimary>
          {primaryAuthType !== 'passwordless' && isPasswordlessEnabled && (
            <Box mt={3} textAlign="center">
              <ButtonText
                onClick={e => switchToLocalFlow(e, true)}
                disabled={submitAttempt.status === 'processing'}
              >
                Back
              </ButtonText>
            </Box>
          )}
          {primaryAuthType === 'passwordless' && (
            <Box mt={3} textAlign="center">
              <ButtonText
                onClick={e => switchToLocalFlow(e)}
                disabled={submitAttempt.status === 'processing'}
              >
                Use Password
              </ButtonText>
            </Box>
          )}
        </OnboardCard>
      )}
    </Validation>
  );
}
