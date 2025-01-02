import React, { useMemo, useState } from 'react';

import { Box, ButtonPrimary, Card, Flex, Text } from 'design';
import { OutlineDanger } from 'design/Alert/Alert';
import { StepHeader } from 'design/StepSlider';
import FieldInput from 'shared/components/FieldInput';
import FieldSelect from 'shared/components/FieldSelect';
import Validation, { Validator } from 'shared/components/Validation';
import {
  requiredField,
  requiredPassword,
} from 'shared/components/Validation/rules';
import { Auth2faType } from 'shared/services';
import createMfaOptions, { MfaOption } from 'shared/utils/createMfaOptions';

import useVerifyUser, { Props, State } from './useVerifyUser';

export default function Container(props: Props) {
  const state = useVerifyUser(props);
  return <VerifyUser {...state} />;
}

function getMethodDescription(auth2faType: Auth2faType) {
  switch (auth2faType) {
    case 'on':
      return 'multi-factor device';
    case 'otp':
      return 'authenticator app';
    case 'webauthn':
      return 'passkey or security key';
    default:
      return 'unknown device type';
  }
}

export function VerifyUser({
  attempt,
  token,
  submitPasswordCreds,
  submitTotpCreds,
  submitWebauthnCreds,
  auth2faType,
  preferredMfaType,
}: State) {
  const [password, setPassword] = useState('');
  const [otpToken, setOtpToken] = useState('');
  const { username, isRecoverPassword } = token;

  const mfaOptions = useMemo<MfaOption[]>(() => {
    if (isRecoverPassword) {
      return createMfaOptions({
        auth2faType: auth2faType,
        preferredType: preferredMfaType,
      });
    }
    return [];
  }, [isRecoverPassword]);

  const [mfaOption, setMfaOption] = useState<MfaOption>(mfaOptions[0]);

  function onSubmitCreds(
    e: React.MouseEvent<HTMLButtonElement>,
    validator: Validator
  ) {
    e.preventDefault();
    if (!validator.validate()) {
      return;
    }

    if (isRecoverPassword) {
      switch (mfaOption.value) {
        case 'otp':
          submitTotpCreds(otpToken);
          break;
        case 'webauthn':
          submitWebauthnCreds();
          break;
      }
    } else {
      submitPasswordCreds(password);
    }
  }

  const title = isRecoverPassword
    ? 'Password Recovery'
    : 'Multi-Factor Device Recovery';

  const instructions = `Verify your identity using your ${
    isRecoverPassword ? getMethodDescription(auth2faType) : 'password'
  }.`;

  return (
    <Card as="form" mx="auto" width="512px" p={4}>
      <Validation>
        {({ validator }) => (
          <>
            <Box mb={4}>
              <StepHeader
                flowLength={isRecoverPassword ? 3 : 4}
                stepIndex={0}
                title={title}
              />
            </Box>

            <Text mb={3} color="text.slightlyMuted">
              {instructions}
            </Text>
            {attempt.status === 'failed' && (
              <OutlineDanger width="100%">{attempt.statusText}</OutlineDanger>
            )}
            <FieldInput
              label="Username"
              value={username}
              onChange={() => null}
              readonly
              mb={3}
            />
            {!isRecoverPassword ? (
              <FieldInput
                rule={requiredPassword}
                label="Password"
                placeholder="Password"
                value={password}
                type="password"
                onChange={e => setPassword(e.target.value)}
                readonly={attempt.status === 'processing'}
                mb={3}
              />
            ) : (
              <Flex alignItems="start">
                <FieldSelect
                  maxWidth="50%"
                  width="100%"
                  label="Multi-Factor Type"
                  value={mfaOption}
                  options={mfaOptions}
                  onChange={(o: MfaOption) => setMfaOption(o)}
                  mr={3}
                  isDisabled={attempt.status === 'processing'}
                  elevated={true}
                  mb={3}
                />
                {mfaOption.value === 'otp' && (
                  <FieldInput
                    width="50%"
                    label="Authenticator Code"
                    rule={requiredField('Authenticator Code is required')}
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    value={otpToken}
                    onChange={e => setOtpToken(e.target.value)}
                    placeholder="123 456"
                    readonly={attempt.status === 'processing'}
                    mb={3}
                  />
                )}
              </Flex>
            )}
            <ButtonPrimary
              size="large"
              width="100%"
              type="submit"
              onClick={e => onSubmitCreds(e, validator)}
              disabled={attempt.status === 'processing'}
            >
              Continue
            </ButtonPrimary>
          </>
        )}
      </Validation>
    </Card>
  );
}
