import { OutlineDanger } from 'design/Alert/Alert';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import { RadioGroup } from 'design/RadioGroup';
import React, { useState, FormEvent } from 'react';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { Auth2faType } from 'shared/services';
import createMfaOptions from 'shared/utils/createMfaOptions';
import { StepComponentProps, StepHeader } from 'design/StepSlider';

import Box from 'design/Box';

import useReAuthenticate from 'teleport/components/ReAuthenticate/useReAuthenticate';

export type ReauthenticateStepProps = StepComponentProps & {
  auth2faType: Auth2faType;
  onAuthenticated(privilegeToken: string): void;
  onClose(): void;
};

export function ReauthenticateStep({
  next,
  refCallback,
  stepIndex,
  flowLength,
  auth2faType,
  onClose,
  onAuthenticated: onAuthenticatedProp,
}: ReauthenticateStepProps) {
  const onAuthenticated = (privilegeToken: string) => {
    onAuthenticatedProp(privilegeToken);
    next();
  };
  const { attempt, clearAttempt, submitWithTotp, submitWithWebauthn } =
    useReAuthenticate({
      onAuthenticated,
    });
  const mfaOptions = createMfaOptions({
    auth2faType,
    required: true,
  });

  const [mfaOption, setMfaOption] = useState<Auth2faType>(mfaOptions[0].value);
  const [authCode, setAuthCode] = useState('');

  const onAuthCodeChanged = (e: React.ChangeEvent<HTMLInputElement>) => {
    setAuthCode(e.target.value);
  };

  const onReauthenticate = (
    e: FormEvent<HTMLFormElement>,
    validator: Validator
  ) => {
    e.preventDefault();
    if (!validator.validate()) return;
    if (mfaOption === 'webauthn') {
      submitWithWebauthn();
    }
    if (mfaOption === 'otp') {
      submitWithTotp(authCode);
    }
  };

  // This message relies on the status message produced by the auth server in
  // lib/auth/Server.checkOTP function. Please keep these in sync.
  const errorMessage =
    attempt.statusText === 'invalid totp token'
      ? 'Invalid authenticator code'
      : attempt.statusText;

  return (
    <div ref={refCallback} data-testid="reauthenticate-step">
      <Box mb={4}>
        <StepHeader
          stepIndex={stepIndex}
          flowLength={flowLength}
          title="Verify Identity"
        />
      </Box>
      {attempt.status === 'failed' && (
        <OutlineDanger>{errorMessage}</OutlineDanger>
      )}
      Multi-factor type
      <Validation>
        {({ validator }) => (
          <form onSubmit={e => onReauthenticate(e, validator)}>
            <RadioGroup
              name="mfaOption"
              options={mfaOptions}
              value={mfaOption}
              autoFocus
              flexDirection="row"
              gap={3}
              mb={4}
              onChange={o => {
                setMfaOption(o as Auth2faType);
                clearAttempt();
              }}
            />
            {mfaOption === 'otp' && (
              <FieldInput
                label="Authenticator Code"
                rule={requiredField('Authenticator code is required')}
                inputMode="numeric"
                autoComplete="one-time-code"
                value={authCode}
                placeholder="123 456"
                onChange={onAuthCodeChanged}
                readonly={attempt.status === 'processing'}
              />
            )}
            <Flex gap={2}>
              <ButtonPrimary type="submit" block={true}>
                Verify my identity
              </ButtonPrimary>
              <ButtonSecondary type="button" block={true} onClick={onClose}>
                Cancel
              </ButtonSecondary>
            </Flex>
          </form>
        )}
      </Validation>
    </div>
  );
}
