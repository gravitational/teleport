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

import React, { useState } from 'react';

import { Box, ButtonPrimary, Flex, Image, Text } from 'design';
import { Danger } from 'design/Alert';
import { ArrowBack } from 'design/Icon';
import { RadioGroup } from 'design/RadioGroup';
import { StepHeader } from 'design/StepSlider';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { useRefAutoFocus } from 'shared/hooks';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { Auth2faType } from 'shared/services';
import createMfaOptions from 'shared/utils/createMfaOptions';

import { OnboardCard } from 'teleport/components/Onboard';
import { PasskeyIcons } from 'teleport/components/Passkeys';

export interface NewMfaDeviceFormProps {
  title: string;
  submitButtonText: string;
  submitAttempt: Attempt;
  clearSubmitAttempt: () => void;
  qrCode: string;
  /** Determines available MFA options. */
  auth2faType: Auth2faType;
  /** A client-side WebAuthn credential, if already created. */
  credential?: Credential;
  /** Creates a new WebAuthn credential on the client side. */
  createNewWebAuthnDevice: () => void;
  /** Submits a new WebAuthn MFA method to the auth server. */
  onSubmitWithWebAuthn: (deviceName: string) => void;
  /** Submits a new OTP MFA method to the auth server. */
  onSubmit: (otpCode: string, deviceName: string) => void;
  /** Switches to the previous step. */
  prev?: () => void;
  shouldFocus: boolean;
  stepIndex: number;
  flowLength: number;
}

/**
 * Renders a form for adding MFA methods, used in account setup, reset, and
 * recovery scenarios.
 */
export function NewMfaDeviceForm({
  title,
  submitButtonText,
  submitAttempt,
  clearSubmitAttempt,
  qrCode,
  auth2faType,
  credential,
  createNewWebAuthnDevice,
  onSubmitWithWebAuthn,
  onSubmit,
  prev,
  shouldFocus,
  stepIndex,
  flowLength,
}: NewMfaDeviceFormProps) {
  const [otp, setOtp] = useState('');
  const mfaOptions = createMfaOptions({
    auth2faType: auth2faType as Auth2faType,
  });
  const [mfaType, setMfaType] = useState(mfaOptions[0]);
  const [deviceName, setDeviceName] = useState(() =>
    getDefaultDeviceName(mfaType.value)
  );

  const deviceNameInputRef = useRefAutoFocus<HTMLInputElement>({
    shouldFocus,
    refocusDeps: [mfaType.value],
  });

  /**
   * Depending on the selected MFA option and whether the WebAuthn client-side
   * credential has already been created, either creates WebAuthn credentials on
   * the client side and switches the state to ask for device name, or submits
   * the MFA method to the server.
   */
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
        if (!credential) {
          createNewWebAuthnDevice();
        } else {
          onSubmitWithWebAuthn(deviceName);
        }
        break;
      default:
        onSubmit(otp, deviceName);
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

  const qrCodeImage = `data:image/png;base64,${qrCode}`;

  return (
    <Validation>
      {({ validator }) => (
        <OnboardCard>
          <Flex alignItems="center" mb={4}>
            {prev && (
              <ArrowBack
                role="button"
                aria-label="Back"
                size="large"
                mr={3}
                onClick={() => {
                  clearSubmitAttempt();
                  prev();
                }}
                style={{ cursor: 'pointer' }}
              />
            )}
            <StepHeader
              stepIndex={stepIndex}
              flowLength={flowLength}
              title={title}
            />
          </Flex>
          {submitAttempt.status === 'failed' && (
            <Danger children={submitAttempt.statusText} />
          )}
          <Text typography="body2" color="text.main" mb={1}>
            Multi-factor type
          </Text>
          <Box mb={3}>
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
            borderRadius={8}
            bg={mfaType?.value === 'optional' ? 'levels.elevated' : ''}
            gap={3}
            mb={3}
          >
            {mfaType?.value === 'webauthn' && (
              <Box
                border={1}
                borderColor="interactive.tonal.neutral.2"
                borderRadius={3}
                p={3}
              >
                <PasskeyIcons />
                <Text mt={2}>
                  You can use Touch ID, Face ID, Windows Hello, a hardware
                  device, or an authenticator app as an MFA method.
                </Text>
              </Box>
            )}
            {(mfaType?.value === 'otp' ||
              (mfaType?.value === 'webauthn' && !!credential)) && (
              <FieldInput
                rule={requiredField('MFA method name is required')}
                label="MFA method name"
                placeholder="Name"
                ref={deviceNameInputRef}
                value={deviceName}
                type="text"
                onChange={e => setDeviceName(e.target.value)}
                readonly={submitAttempt.status === 'processing'}
                mb={0}
              />
            )}
            {mfaType?.value === 'otp' && (
              <Flex
                border={1}
                borderColor="interactive.tonal.neutral.2"
                borderRadius={3}
                p={3}
                gap={3}
              >
                <Image
                  src={qrCodeImage}
                  width="168px"
                  height="168px"
                  css={`
                    border: 4px solid white;
                    box-sizing: border-box;
                    border: 8px solid white;
                    border-radius: 8px;
                  `}
                />
                <Flex flexDirection="column">
                  <Box flex="1">
                    <Text>
                      Scan the QR Code with any authenticator app and enter the
                      generated code.
                    </Text>
                  </Box>
                  <FieldInput
                    label="Authenticator Code"
                    rule={requiredField('Authenticator code is required')}
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    value={otp}
                    onChange={e => setOtp(e.target.value)}
                    placeholder="123 456"
                    readonly={submitAttempt.status === 'processing'}
                    mb={0}
                  />
                </Flex>
              </Flex>
            )}
            {mfaType?.value === 'optional' && (
              <Text textAlign="center" p={5}>
                We strongly recommend enrolling a multi-factor authentication
                method to protect both yourself and your organization.
              </Text>
            )}
          </Flex>
          <ButtonPrimary
            width="100%"
            disabled={submitAttempt.status === 'processing'}
            size="large"
            onClick={e => onBtnClick(e, validator)}
          >
            {mfaType.value === 'webauthn' && !credential
              ? 'Create an MFA Method'
              : submitButtonText}
          </ButtonPrimary>
        </OnboardCard>
      )}
    </Validation>
  );
}

export function getDefaultDeviceName(mfaType: Auth2faType) {
  if (mfaType === 'webauthn') {
    return 'webauthn-device';
  }
  if (mfaType === 'otp') {
    return 'otp-device';
  }
  return '';
}
