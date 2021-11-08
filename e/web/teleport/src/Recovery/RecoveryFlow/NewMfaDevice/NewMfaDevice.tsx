import React, { useState, useMemo } from 'react';
import { Card, ButtonPrimary, Text, Flex, Image, Box, Link } from 'design';
import { Danger } from 'design/Alert';
import FieldInput from 'shared/components/FieldInput';
import FieldSelect from 'shared/components/FieldSelect';
import Validation, { Validator } from 'shared/components/Validation';
import {
  requiredToken,
  requiredField,
} from 'shared/components/Validation/rules';
import { getMfaOptions, MfaOption } from 'teleport/services/mfa/utils';
import useNewMfaDevice, { State, Props } from './useNewMfaDevice';

const u2fGraphic = require('design/assets/images/u2f-graphic.svg');

export default function Container(props: Props) {
  const state = useNewMfaDevice(props);
  return <NewMfaDevice {...state} />;
}

export function NewMfaDevice({
  attempt,
  clearSubmitAttempt,
  qrCode,
  setNewTotpDevice,
  setNewU2fDevice,
  setNewWebauthnDevice,
  auth2faType,
  preferredMfaType,
}: State) {
  const [otpToken, setOtpToken] = useState('');
  const [deviceName, setDeviceName] = useState('');

  const mfaOptions = useMemo<MfaOption[]>(
    () => getMfaOptions(auth2faType, preferredMfaType),
    []
  );

  const [mfaOption, setMfaOption] = useState<MfaOption>(mfaOptions[0]);

  function onSetMfaOption(option: MfaOption, validator: Validator) {
    setOtpToken('');
    clearSubmitAttempt();
    validator.reset();
    setMfaOption(option);
  }

  function onSubmit(
    e: React.MouseEvent<HTMLButtonElement>,
    validator: Validator
  ) {
    e.preventDefault();
    if (!validator.validate()) {
      return;
    }

    if (mfaOption.value === 'otp') {
      setNewTotpDevice(otpToken, deviceName);
    } else if (mfaOption.value === 'u2f') {
      setNewU2fDevice(deviceName);
    } else if (mfaOption.value === 'webauthn') {
      setNewWebauthnDevice(deviceName);
    }
  }

  const imgSrc =
    mfaOption.value === 'otp' ? `data:image/png;base64,${qrCode}` : u2fGraphic;

  let hardwareInstructions = 'Enter a name for your hardware key.';
  if (attempt.status === 'processing') {
    hardwareInstructions =
      mfaOption.value === 'u2f'
        ? 'Insert your new hardware key and press the button on the key.'
        : 'Follow the prompts from your browser.';
  }

  return (
    <Card as="form" bg="primary.light" mx="auto" width="512px">
      <Validation>
        {({ validator }) => (
          <>
            <Text typography="h3" pt={5} textAlign="center" color="light">
              Enroll New Two-Factor Device
            </Text>
            <Text textAlign="center" color="text.secondary">
              Step 2 of 4
            </Text>
            <Box p={5}>
              {attempt.status === 'failed' && (
                <Danger width="100%">{attempt.statusText}</Danger>
              )}
              <Flex
                flexDirection="column"
                justifyContent="center"
                alignItems="center"
                bg="primary.main"
                borderRadius={8}
                height="256px"
                p={3}
                mb={4}
              >
                <Image
                  src={imgSrc}
                  width="152px"
                  style={{
                    border: mfaOption.value === 'otp' ? '8px solid' : 'none',
                  }}
                />
                {mfaOption.value === 'otp' && (
                  <Text fontSize={1} textAlign="center" mt={2}>
                    Scan the QR Code with any authenticator app and enter the
                    generated code.{' '}
                    <Text color="text.secondary">
                      We recommend{' '}
                      <Link href="https://authy.com/download/" target="_blank">
                        Authy
                      </Link>
                      .
                    </Text>
                  </Text>
                )}
                {(mfaOption.value === 'u2f' ||
                  mfaOption.value === 'webauthn') && (
                  <Text mt={3}>{hardwareInstructions}</Text>
                )}
              </Flex>
              <Flex alignItems="center">
                <FieldSelect
                  maxWidth="50%"
                  width="100%"
                  label="Two-factor type"
                  value={mfaOption}
                  options={mfaOptions}
                  onChange={(o: MfaOption) => onSetMfaOption(o, validator)}
                  mr={3}
                  isDisabled={attempt.status === 'processing'}
                />
                {mfaOption.value === 'otp' && (
                  <FieldInput
                    width="50%"
                    label="Authenticator code"
                    rule={requiredToken}
                    autoComplete="off"
                    value={otpToken}
                    onChange={e => setOtpToken(e.target.value)}
                    placeholder="123 456"
                    readonly={attempt.status === 'processing'}
                  />
                )}
              </Flex>
              <FieldInput
                rule={requiredField('Device name is required')}
                label="Device name"
                placeholder="Name"
                width="100%"
                autoFocus
                value={deviceName}
                type="text"
                onChange={e => setDeviceName(e.target.value)}
                readonly={attempt.status === 'processing'}
              />
              <ButtonPrimary
                mt={3}
                size="large"
                width="100%"
                type="submit"
                onClick={e => onSubmit(e, validator)}
                disabled={attempt.status === 'processing'}
              >
                Continue
              </ButtonPrimary>
            </Box>
          </>
        )}
      </Validation>
    </Card>
  );
}
