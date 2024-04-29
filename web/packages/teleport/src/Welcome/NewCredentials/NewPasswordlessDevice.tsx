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
import { Text, Box, ButtonPrimary, ButtonText } from 'design';
import { Danger, Info } from 'design/Alert';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { useRefAutoFocus } from 'shared/hooks';

import { OnboardCard } from 'design/Onboard/OnboardCard';

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

  // Firefox currently does not support passwordless and when
  // registering, users will 'soft lock' where firefox prompts
  // but when touching the device, it does not do anything.
  // We display a soft warning because firefox may provide
  // support in the near future: https://github.com/gravitational/webapps/pull/876
  const isFirefox = window.navigator?.userAgent
    ?.toLowerCase()
    .includes('firefox');

  return (
    <Validation>
      {({ validator }) => (
        <OnboardCard ref={refCallback} data-testid="passwordless">
          <Text typography="h4" mb={3} color="text.main" bold>
            Set up Passwordless Authentication
          </Text>
          {submitAttempt.status === 'failed' && (
            <Danger children={submitAttempt.statusText} />
          )}
          {isFirefox && (
            <Info mt={3}>
              Firefox may not support passwordless register. Please try Chrome
              or Safari.
            </Info>
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
            {credential ? 'Submit' : 'Create a passkey'}
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
                Use password
              </ButtonText>
            </Box>
          )}
        </OnboardCard>
      )}
    </Validation>
  );
}
