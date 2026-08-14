import React, { useMemo, useState } from 'react';

import { Box, ButtonPrimary, Card, Flex, Indicator, Text } from 'design';
import { OutlineDanger } from 'design/Alert/Alert';
import { StepHeader } from 'design/StepSlider';
import FieldInput from 'shared/components/FieldInput';
import FieldSelect from 'shared/components/FieldSelect';
import Validation, { Validator } from 'shared/components/Validation';
import {
  requiredField,
  requiredPassword,
} from 'shared/components/Validation/rules';

import auth from 'teleport/services/auth';
import {
  getMfaChallengeOptions,
  MfaAuthenticateChallenge,
  MfaOption,
} from 'teleport/services/mfa';
import { createQueryHook } from 'teleport/services/queryHelpers';

import useVerifyUser, { Props, State } from './useVerifyUser';

export default function Container(props: Props) {
  const state = useVerifyUser(props);
  return <VerifyUser {...state} />;
}

// Fetches the MFA challenge for an account recovery token.
const { useQuery: useGetRecoveryMfaChallenge } = createQueryHook(
  ['recovery', 'mfaChallenge'],
  (tokenId: string) => auth.createMfaAuthnChallengeWithToken(tokenId)
);

function getMethodDescription(challenge: MfaAuthenticateChallenge | undefined) {
  const hasWebauthn = !!challenge?.webauthnPublicKey;
  const hasTotp = !!challenge?.totpChallenge;
  if (hasWebauthn && hasTotp) return 'multi-factor device';
  if (hasWebauthn) return 'passkey or security key';
  if (hasTotp) return 'authenticator app';
  return 'unknown device type';
}

export function VerifyUser({
  token,
  submitAttempt,
  submitWithPassword,
  submitWithMfa,
}: State) {
  const [password, setPassword] = useState('');
  const [otpToken, setOtpToken] = useState('');
  const [mfaOption, setMfaOption] = useState<MfaOption>();
  const { username, isRecoverPassword } = token;

  const challengeQuery = useGetRecoveryMfaChallenge(token.id, {
    enabled: isRecoverPassword,
  });

  const mfaOptions = useMemo<MfaOption[]>(() => {
    if (!challengeQuery.isSuccess) {
      return [];
    }
    return getMfaChallengeOptions(challengeQuery.data);
  }, [challengeQuery.isSuccess, challengeQuery.data]);

  if (mfaOptions.length && !mfaOption) {
    setMfaOption(mfaOptions[0]);
  }

  function onSubmitCreds(
    e: React.MouseEvent<HTMLButtonElement>,
    validator: Validator
  ) {
    e.preventDefault();
    if (!validator.validate()) {
      return;
    }

    if (!isRecoverPassword) {
      submitWithPassword(password);
      return;
    }

    if (mfaOption && challengeQuery.data) {
      submitWithMfa(challengeQuery.data, mfaOption.value, otpToken);
    }
  }

  const title = isRecoverPassword
    ? 'Password Recovery'
    : 'Multi-Factor Device Recovery';

  const instructions = `Verify your identity using your ${
    isRecoverPassword ? getMethodDescription(challengeQuery.data) : 'password'
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
            {submitAttempt.status === 'failed' && (
              <OutlineDanger width="100%">
                {submitAttempt.statusText}
              </OutlineDanger>
            )}
            {challengeQuery.isError && (
              <OutlineDanger width="100%">
                {challengeQuery.error.message}
              </OutlineDanger>
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
                readonly={submitAttempt.status === 'processing'}
                mb={3}
              />
            ) : challengeQuery.isPending ? (
              <Box textAlign="center" mb={3}>
                <Indicator />
              </Box>
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
                  isDisabled={submitAttempt.status === 'processing'}
                  elevated={true}
                  mb={3}
                />
                {mfaOption?.value === 'totp' && (
                  <FieldInput
                    width="50%"
                    label="Authenticator Code"
                    rule={requiredField('Authenticator Code is required')}
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    value={otpToken}
                    onChange={e => setOtpToken(e.target.value)}
                    placeholder="123 456"
                    readonly={submitAttempt.status === 'processing'}
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
              disabled={
                submitAttempt.status === 'processing' ||
                (isRecoverPassword && !mfaOption)
              }
            >
              Continue
            </ButtonPrimary>
          </>
        )}
      </Validation>
    </Card>
  );
}
