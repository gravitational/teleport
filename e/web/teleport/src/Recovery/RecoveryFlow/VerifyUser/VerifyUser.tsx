import React, { useState, useMemo } from 'react';
import { Card, Text, ButtonPrimary, Box, Flex } from 'design';
import { Danger } from 'design/Alert';
import { Auth2faType } from 'shared/services';
import Validation, { Validator } from 'shared/components/Validation';
import FieldInput from 'shared/components/FieldInput';
import {
  requiredPassword,
  requiredToken,
} from 'shared/components/Validation/rules';
import FieldSelect from 'shared/components/FieldSelect';
import { getMfaOptions, MfaOption } from 'teleport/services/mfa/utils';
import useVerifyUser, { State, Props } from './useVerifyUser';

export default function Container(props: Props) {
  const state = useVerifyUser(props);
  return <VerifyUser {...state} />;
}

function getMethodDescription(auth2faType: Auth2faType) {
  switch (auth2faType) {
    case 'on':
      return 'two-factor device';
    case 'otp':
      return 'authenticator app';
    case 'u2f' || 'webauthn':
      return 'hardware key';
    default:
      return 'unknown device type';
  }
}

export function VerifyUser({
  attempt,
  token,
  submitPasswordCreds,
  submitTotpCreds,
  submitU2fCreds,
  submitWebauthnCreds,
  auth2faType,
  preferredMfaType,
}: State) {
  const [password, setPassword] = useState('');
  const [otpToken, setOtpToken] = useState('');
  const { username, isRecoverPassword } = token;

  const mfaOptions = useMemo<MfaOption[]>(() => {
    if (isRecoverPassword) {
      return getMfaOptions(auth2faType, preferredMfaType);
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
        case 'u2f':
          submitU2fCreds();
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
    : 'Two-Factor Device Recovery';

  const instructions = `Verify your identity using your ${
    isRecoverPassword ? getMethodDescription(auth2faType) : 'password'
  }.`;

  return (
    <Card as="form" bg="primary.light" mx="auto" width="512px">
      <Validation>
        {({ validator }) => (
          <>
            <Text typography="h3" pt={5} textAlign="center" color="light">
              {title}
            </Text>
            <Text textAlign="center" color="text.secondary">
              Step 1 of {isRecoverPassword ? 3 : 4}
            </Text>
            <Box p={5}>
              <Text mb={3} textAlign="center" color="text.secondary">
                {instructions}
              </Text>
              {attempt.status === 'failed' && (
                <Danger width="100%">{attempt.statusText}</Danger>
              )}
              <FieldInput
                label="Username"
                value={username}
                onChange={() => null}
                readonly
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
                />
              ) : (
                <Flex alignItems="center">
                  <FieldSelect
                    maxWidth="50%"
                    width="100%"
                    label="Two-factor type"
                    value={mfaOption}
                    options={mfaOptions}
                    onChange={(o: MfaOption) => setMfaOption(o)}
                    mr={3}
                    isDisabled={attempt.status === 'processing'}
                  />
                  {mfaOption.value === 'otp' && (
                    <FieldInput
                      width="50%"
                      label="two-factor token"
                      rule={requiredToken}
                      autoComplete="off"
                      value={otpToken}
                      onChange={e => setOtpToken(e.target.value)}
                      placeholder="123 456"
                      readonly={attempt.status === 'processing'}
                    />
                  )}
                  {mfaOption.value === 'u2f' &&
                    attempt.status === 'processing' && (
                      <Text typography="body2">
                        Insert your hardware key and press the button on the
                        key.
                      </Text>
                    )}
                </Flex>
              )}
              <ButtonPrimary
                mt={3}
                size="large"
                width="100%"
                type="submit"
                onClick={e => onSubmitCreds(e, validator)}
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
