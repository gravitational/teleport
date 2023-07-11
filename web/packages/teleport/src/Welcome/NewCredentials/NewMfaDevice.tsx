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
import { Box, ButtonPrimary, Flex, Image, Link, Text } from 'design';
import { Danger } from 'design/Alert';
import { ArrowBack } from 'design/Icon';
import { RadioGroup } from 'design/RadioGroup';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import {
  requiredField,
  requiredToken,
} from 'shared/components/Validation/rules';
import createMfaOptions from 'shared/utils/createMfaOptions';

import { useRefAutoFocus } from 'shared/hooks';
import { Auth2faType } from 'shared/services';

import {
  SliderProps,
  UseTokenState,
} from 'teleport/Welcome/NewCredentials/types';

import secKeyGraphic from './sec-key-with-bg.png';

export function NewMfaDevice(props: Props) {
  const {
    resetToken,
    submitAttempt,
    clearSubmitAttempt,
    auth2faType,
    onSubmitWithWebauthn,
    onSubmit,
    password,
    prev,
    refCallback,
    hasTransitionEnded,
  } = props;
  const [otp, setOtp] = useState('');
  const mfaOptions = createMfaOptions({
    auth2faType: auth2faType as Auth2faType,
  });
  const [mfaType, setMfaType] = useState(mfaOptions[0]);
  const [deviceName, setDeviceName] = useState(() =>
    getDefaultDeviceName(mfaType.value)
  );

  const deviceNameInputRef = useRefAutoFocus<HTMLInputElement>({
    shouldFocus: hasTransitionEnded,
    refocusDeps: [mfaType.value],
  });

  function onBtnClick(
    e: React.MouseEvent<HTMLButtonElement>,
    validator: Validator
  ) {
    e.preventDefault(); // prevent form submit default

    if (!validator.validate()) {
      return;
    }

    switch (mfaType?.value) {
      case 'webauthn':
        onSubmitWithWebauthn(password, deviceName);
        break;
      default:
        onSubmit(password, otp, deviceName);
    }
  }

  function onSetMfaOption(value: string, validator: Validator) {
    setOtp('');
    clearSubmitAttempt();
    validator.reset();

    const mfaOpt = mfaOptions.find(option => option.value === value);
    if (mfaOpt) {
      setMfaType(mfaOpt);
      setDeviceName(getDefaultDeviceName(mfaOpt.value));
    }
  }

  const imgSrc =
    mfaType?.value === 'otp'
      ? `data:image/png;base64,${resetToken.qrCode}`
      : secKeyGraphic;

  return (
    <Validation>
      {({ validator }) => (
        <Box p={5} ref={refCallback}>
          <Flex mb={3} alignItems="center">
            <ArrowBack
              fontSize={30}
              mr={3}
              onClick={() => {
                clearSubmitAttempt();
                prev();
              }}
              style={{ cursor: 'pointer' }}
            />
            <Box>
              <Text color="text.slightlyMuted">Step 2 of 2</Text>
              <Text typography="h4" color="text.main" bold>
                Set Two-Factor Device
              </Text>
            </Box>
          </Flex>
          {submitAttempt.status === 'failed' && (
            <Danger children={submitAttempt.statusText} />
          )}
          <Text typography="subtitle1" color="text.main" caps mb={1}>
            Two-Factor Method
          </Text>
          <Box mb={1}>
            <RadioGroup
              name="mfaType"
              options={mfaOptions}
              value={mfaType.value}
              flexDirection="row"
              gap="16px"
              onChange={value => onSetMfaOption(value, validator)}
            />
          </Box>
          <Flex
            flexDirection="column"
            justifyContent="center"
            alignItems="center"
            borderRadius={8}
            bg={mfaType?.value === 'optional' ? 'levels.elevated' : ''}
            height={mfaType?.value === 'optional' ? '340px' : '240px'}
            px={3}
          >
            {mfaType?.value === 'otp' && (
              <>
                <Image
                  src={imgSrc}
                  width="145px"
                  height="145px"
                  css={`
                    border: 4px solid white;
                  `}
                />
                <Text
                  fontSize={1}
                  textAlign="center"
                  mt={2}
                  color="text.slightlyMuted"
                >
                  Scan the QR Code with any authenticator app and enter the
                  generated code. We recommend{' '}
                  <Link href="https://authy.com/download/" target="_blank">
                    Authy
                  </Link>
                  .
                </Text>
              </>
            )}
            {mfaType?.value === 'webauthn' && (
              <>
                <Image src={imgSrc} width="220px" height="154px" />
                <Text
                  fontSize={1}
                  color="text.slightlyMuted"
                  textAlign="center"
                >
                  We support a wide range of hardware devices including
                  YubiKeys, Touch ID, watches, and more.
                </Text>
              </>
            )}
            {mfaType?.value === 'optional' && (
              <Text textAlign="center">
                We strongly recommend enrolling a two-factor device to protect
                both yourself and your organization.
              </Text>
            )}
          </Flex>
          {mfaType?.value !== 'optional' && (
            <Flex alignItems="center" height={100}>
              <FieldInput
                rule={requiredField('Device name is required')}
                label="Device name"
                placeholder="Name"
                ref={deviceNameInputRef}
                width={mfaType?.value === 'otp' ? '50%' : '100%'}
                value={deviceName}
                type="text"
                onChange={e => setDeviceName(e.target.value)}
                readonly={submitAttempt.status === 'processing'}
                mr={mfaType?.value === 'otp' ? 3 : 0}
              />
              {mfaType?.value === 'otp' && (
                <FieldInput
                  width="50%"
                  label="Authenticator code"
                  rule={requiredToken}
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  value={otp}
                  onChange={e => setOtp(e.target.value)}
                  placeholder="123 456"
                  readonly={submitAttempt.status === 'processing'}
                />
              )}
            </Flex>
          )}
          <ButtonPrimary
            width="100%"
            mt={2}
            disabled={submitAttempt.status === 'processing'}
            size="large"
            onClick={e => onBtnClick(e, validator)}
          >
            Submit
          </ButtonPrimary>
        </Box>
      )}
    </Validation>
  );
}

function getDefaultDeviceName(mfaType: Auth2faType) {
  if (mfaType === 'webauthn') {
    return 'webauthn-device';
  }
  if (mfaType === 'otp') {
    return 'otp-device';
  }
  return '';
}

type Props = UseTokenState &
  SliderProps & {
    password: string;
    updatePassword(pwd: string): void;
  };
