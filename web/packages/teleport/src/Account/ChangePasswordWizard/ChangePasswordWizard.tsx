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

import React, { useCallback, useEffect, useState } from 'react';
import styled from 'styled-components';

import { Alert, OutlineDanger } from 'design/Alert/Alert';
import Box from 'design/Box';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Dialog from 'design/Dialog';
import Flex from 'design/Flex';
import Indicator from 'design/Indicator';
import { RadioGroup } from 'design/RadioGroup';
import { StepComponentProps, StepHeader, StepSlider } from 'design/StepSlider';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import {
  requiredConfirmedPassword,
  requiredField,
  requiredPassword,
} from 'shared/components/Validation/rules';
import { useAsync } from 'shared/hooks/useAsync';

import useReAuthenticate, {
  ReauthState,
} from 'teleport/components/ReAuthenticate/useReAuthenticate';
import { ChangePasswordReq } from 'teleport/services/auth';
import auth, { MfaChallengeScope } from 'teleport/services/auth/auth';
import {
  DeviceType,
  MfaOption,
  WebauthnAssertionResponse,
} from 'teleport/services/mfa';

export interface ChangePasswordWizardProps {
  hasPasswordless: boolean;
  onClose(): void;
  onSuccess(): void;
}

export function ChangePasswordWizard({
  hasPasswordless,
  onClose,
  onSuccess,
}: ChangePasswordWizardProps) {
  const [webauthnResponse, setWebauthnResponse] =
    useState<WebauthnAssertionResponse>();

  const reauthState = useReAuthenticate({
    challengeScope: MfaChallengeScope.CHANGE_PASSWORD,
    onMfaResponse: async mfaResponse =>
      setWebauthnResponse(mfaResponse.webauthn_response),
  });

  const [reauthMethod, setReauthMethod] = useState<ReauthenticationMethod>();

  return (
    <Dialog
      open={true}
      disableEscapeKeyDown={false}
      dialogCss={() => ({ width: '650px', padding: 0 })}
      onClose={onClose}
    >
      <StepSlider
        flows={wizardFlows}
        currFlow={'withReauthentication'}
        // Step properties
        hasPasswordless={hasPasswordless}
        reauthMethod={reauthMethod}
        setReauthMethod={setReauthMethod}
        reauthState={reauthState}
        webauthnResponse={webauthnResponse}
        onClose={onClose}
        onSuccess={onSuccess}
      />
    </Dialog>
  );
}

type ReauthenticationMethod = 'passwordless' | DeviceType;
type ReauthenticationOption = {
  value: ReauthenticationMethod;
  label: string;
};

export const REAUTH_OPTION_WEBAUTHN: ReauthenticationOption = {
  value: 'webauthn',
  label: 'Security Key',
};

export const REAUTH_OPTION_PASSKEY: ReauthenticationOption = {
  value: 'passwordless',
  label: 'Passkey',
};

export function getReauthOptions(
  mfaOptions: MfaOption[],
  hasPasswordless: boolean
) {
  // Be more specific about the WebAuthn device type (it's not a passkey).
  const reauthOptions = mfaOptions.map((o: ReauthenticationOption) =>
    o.value === 'webauthn' ? REAUTH_OPTION_WEBAUTHN : o
  );

  // Add passwordless as the default if available.
  if (hasPasswordless) {
    reauthOptions.unshift(REAUTH_OPTION_PASSKEY);
  }

  return reauthOptions;
}

const wizardFlows = {
  withReauthentication: [ReauthenticateStep, ChangePasswordStep],
};

export type ChangePasswordWizardStepProps = StepComponentProps &
  ReauthenticateStepProps &
  ChangePasswordStepProps;

interface ReauthenticateStepProps {
  hasPasswordless: boolean;
  reauthMethod: ReauthenticationMethod;
  setReauthMethod(method: ReauthenticationMethod): void;
  reauthState: ReauthState;
  onClose(): void;
}

export function ReauthenticateStep({
  next,
  refCallback,
  stepIndex,
  flowLength,
  hasPasswordless,
  reauthMethod,
  setReauthMethod,
  reauthState: {
    initAttempt,
    mfaOptions,
    submitWithMfa,
    clearSubmitAttempt,
    submitAttempt,
  },
  onClose,
}: ChangePasswordWizardStepProps) {
  const [reauthOptions, initReauthOptions] = useAsync(
    useCallback(async () => {
      const reauthOptions = getReauthOptions(mfaOptions, hasPasswordless);
      setReauthMethod(reauthOptions[0].value);
      return reauthOptions;
    }, [hasPasswordless, mfaOptions, setReauthMethod])
  );

  useEffect(() => {
    initReauthOptions();
  }, [initReauthOptions]);

  const reauthenticate = useCallback(
    async (reauthMethod: ReauthenticationMethod) => {
      // totp is handled in the ChangePasswordStep
      if (reauthMethod === 'totp') return next();

      const mfaType =
        reauthMethod === 'passwordless' ? 'webauthn' : reauthMethod;
      const deviceUsage =
        reauthMethod === 'passwordless' ? 'passwordless' : 'mfa';

      submitWithMfa(mfaType, deviceUsage).then(([, err]) => {
        if (!err) next();
      });
    },
    [submitWithMfa, next]
  );

  const onReauthenticate = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    reauthenticate(reauthMethod);
  };

  // Handle potential error states first.
  switch (initAttempt.status) {
    case 'processing':
      return (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      );
    case 'error':
      return <Alert children={initAttempt.statusText} />;
    case 'success':
      break;
    default:
      return null;
  }

  // Handle potential error states first.
  switch (reauthOptions.status) {
    case 'processing':
      return (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      );
    case 'error':
      return <Alert children={reauthOptions.statusText} />;
    case 'success':
      break;
    default:
      return null;
  }

  return (
    <StepContainer ref={refCallback} data-testid="reauthenticate-step">
      <Box mb={4}>
        <StepHeader
          stepIndex={stepIndex}
          flowLength={flowLength}
          title="Verify Identity"
        />
      </Box>
      {submitAttempt.status === 'error' && (
        <OutlineDanger>{submitAttempt.statusText}</OutlineDanger>
      )}
      <Box mb={2}>Verification Method</Box>
      <form onSubmit={e => onReauthenticate(e)}>
        <RadioGroup
          name="mfaOption"
          options={reauthOptions.data}
          value={reauthMethod}
          autoFocus
          flexDirection="row"
          gap={3}
          mb={4}
          onChange={o => {
            setReauthMethod(o as DeviceType);
            clearSubmitAttempt();
          }}
        />
        <Flex gap={2}>
          <ButtonPrimary type="submit" block={true}>
            Next
          </ButtonPrimary>
          <ButtonSecondary type="button" block={true} onClick={onClose}>
            Cancel
          </ButtonSecondary>
        </Flex>
      </form>
    </StepContainer>
  );
}

interface ChangePasswordStepProps {
  webauthnResponse: WebauthnAssertionResponse;
  reauthMethod: ReauthenticationMethod;
  onClose(): void;
  onSuccess(): void;
}

export function ChangePasswordStep({
  refCallback,
  prev,
  stepIndex,
  flowLength,
  webauthnResponse,
  reauthMethod,
  onClose,
  onSuccess,
}: ChangePasswordWizardStepProps) {
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [newPassConfirmed, setNewPassConfirmed] = useState('');
  const [otpCode, setOtpCode] = useState('');
  const onAuthCodeChanged = (e: React.ChangeEvent<HTMLInputElement>) => {
    setOtpCode(e.target.value);
  };
  const [changePasswordAttempt, changePassword] = useAsync(
    async (req: ChangePasswordReq) => {
      await auth.changePassword(req);
      // Purge secrets from the state now that they are no longer needed.
      resetForm();
      onSuccess();
    }
  );

  function resetForm() {
    setOldPassword('');
    setNewPassword('');
    setNewPassConfirmed('');
    setOtpCode('');
  }

  async function onSubmit(
    e: React.FormEvent<HTMLFormElement>,
    validator: Validator
  ) {
    e.preventDefault();
    if (!validator.validate()) return;

    await changePassword({
      oldPassword,
      newPassword,
      mfaResponse: {
        totp_code: otpCode,
        webauthn_response: webauthnResponse,
      },
    });
  }

  return (
    <StepContainer ref={refCallback} data-testid="change-password-step">
      <Box mb={4}>
        <StepHeader
          stepIndex={stepIndex}
          flowLength={flowLength}
          title="Change Password"
        />
      </Box>
      <Validation>
        {({ validator }) => (
          <form onSubmit={e => onSubmit(e, validator)}>
            {changePasswordAttempt.status === 'error' && (
              <OutlineDanger>{changePasswordAttempt.statusText}</OutlineDanger>
            )}
            {reauthMethod !== 'passwordless' && (
              <FieldInput
                rule={requiredField('Current Password is required')}
                label="Current Password"
                value={oldPassword}
                onChange={e => setOldPassword(e.target.value)}
                type="password"
                placeholder="Password"
              />
            )}
            <FieldInput
              rule={requiredPassword}
              label="New Password"
              value={newPassword}
              onChange={e => setNewPassword(e.target.value)}
              type="password"
              placeholder="New Password"
            />
            <FieldInput
              rule={requiredConfirmedPassword(newPassword)}
              label="Confirm Password"
              value={newPassConfirmed}
              onChange={e => setNewPassConfirmed(e.target.value)}
              type="password"
              placeholder="Confirm Password"
            />
            {reauthMethod === 'totp' && (
              <FieldInput
                label="Authenticator Code"
                helperText="Enter the code generated by your authenticator app"
                rule={requiredField('Authenticator code is required')}
                inputMode="numeric"
                autoComplete="one-time-code"
                value={otpCode}
                placeholder="123 456"
                onChange={onAuthCodeChanged}
                readonly={changePasswordAttempt.status === 'processing'}
              />
            )}
            <Flex gap={2}>
              <ButtonPrimary type="submit" block={true}>
                Save Changes
              </ButtonPrimary>
              {stepIndex === 0 ? (
                <ButtonSecondary type="button" block={true} onClick={onClose}>
                  Cancel
                </ButtonSecondary>
              ) : (
                <ButtonSecondary type="button" block={true} onClick={prev}>
                  Back
                </ButtonSecondary>
              )}
            </Flex>
          </form>
        )}
      </Validation>
    </StepContainer>
  );
}

/**
 * Sets the padding on the dialog content instead of the dialog itself to make
 * the slide animations reach the dialog border.
 */
const StepContainer = styled.div`
  padding: ${props => props.theme.space[5]}px;
  padding-top: ${props => props.theme.space[4]}px;
`;
