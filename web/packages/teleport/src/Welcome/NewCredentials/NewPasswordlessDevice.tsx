/**
 * Copyright 2022 Gravitational, Inc.
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

import React, { useState } from 'react';
import { Text, Box, ButtonPrimary, ButtonText } from 'design';
import { Danger } from 'design/Alert';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { useRefAutoFocus } from 'shared/hooks';

import { OnboardCard } from 'design/Onboard/OnboardCard';

import { SliderProps, UseTokenState } from './types';

export function NewPasswordlessDevice(props: UseTokenState & SliderProps) {
  const {
    submitAttempt,
    onSubmitWithWebauthn,
    primaryAuthType,
    isPasswordlessEnabled,
    changeFlow,
    refCallback,
    hasTransitionEnded,
    clearSubmitAttempt,
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

    onSubmitWithWebauthn('', deviceName);
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
          <Text typography="h4" mb={3} color="text.main" bold>
            Set A Passwordless Device
          </Text>
          {submitAttempt.status === 'failed' && (
            <Danger children={submitAttempt.statusText} />
          )}
          <FieldInput
            rule={requiredField('Device name is required')}
            label="Device Name"
            placeholder="Name"
            width="100%"
            ref={deviceNameInputRef}
            value={deviceName}
            type="text"
            onChange={e => setDeviceName(e.target.value)}
            readonly={submitAttempt.status === 'processing'}
          />
          <ButtonPrimary
            width="100%"
            mt={1}
            size="large"
            onClick={e => handleOnSubmit(e, validator)}
            disabled={submitAttempt.status === 'processing'}
          >
            Submit
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
