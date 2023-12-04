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

import { OnboardCard } from 'design/Onboard/OnboardCard';

import {
  SliderProps,
  UseTokenState,
} from 'teleport/Welcome/NewCredentials/types';

import secKeyGraphic from './sec-key-with-bg.png';

export function NewMfaDevice(props: NewMfaDeviceProps) {
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
        <OnboardCard ref={refCallback}>
          <Flex mb={3} alignItems="center">
            <ArrowBack
              size="large"
              mr={3}
              onClick={() => {
                clearSubmitAttempt();
                prev();
              }}
              style={{ cursor: 'pointer' }}
            />
            <Box>
              <Text typography="h4" color="text.main" bold>
                Set Two-Factor Device
              </Text>
              <Text color="text.slightlyMuted">Step 2 of 2</Text>
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
                label="Device Name"
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
                  label="Authenticator Code"
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
        </OnboardCard>
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

export type NewMfaDeviceProps = UseTokenState &
  SliderProps & {
    password: string;
    updatePassword(pwd: string): void;
  };
