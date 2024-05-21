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

import styled from 'styled-components';
import { OutlineDanger } from 'design/Alert/Alert';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Dialog from 'design/Dialog';
import Flex from 'design/Flex';
import { RadioGroup } from 'design/RadioGroup';
import { StepComponentProps, StepSlider } from 'design/StepSlider';
import React, { useState } from 'react';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import {
  requiredConfirmedPassword,
  requiredField,
  requiredPassword,
} from 'shared/components/Validation/rules';
import { useAsync } from 'shared/hooks/useAsync';
import { Auth2faType } from 'shared/services';

import Box from 'design/Box';

import { StepHeader } from 'design/StepSlider';

import { ChangePasswordReq } from 'teleport/services/auth';
import auth, { MfaChallengeScope } from 'teleport/services/auth/auth';
import { MfaDevice } from 'teleport/services/mfa';

export interface ChangePasswordWizardProps {
  /** MFA type setting, as configured in the cluster's configuration. */
  auth2faType: Auth2faType;
  /** Determines whether the cluster allows passwordless login. */
  passwordlessEnabled: boolean;
  /** A list of available authentication devices. */
  devices: MfaDevice[];
  onClose(): void;
  onSuccess(): void;
}

export function ChangePasswordWizard({
  auth2faType,
  passwordlessEnabled,
  devices,
  onClose,
  onSuccess,
}: ChangePasswordWizardProps) {
  const reauthOptions = createReauthOptions(
    auth2faType,
    passwordlessEnabled,
    devices
  );
  const [reauthMethod, setReauthMethod] = useState<ReauthenticationMethod>(
    reauthOptions[0]?.value
  );
  const [credential, setCredential] = useState<Credential | undefined>();
  const reauthRequired = reauthOptions.length > 0;

  return (
    <Dialog
      open={true}
      disableEscapeKeyDown={false}
      dialogCss={() => ({ width: '650px', padding: 0 })}
      onClose={onClose}
    >
      <StepSlider
        flows={wizardFlows}
        currFlow={
          reauthRequired ? 'withReauthentication' : 'withoutReauthentication'
        }
        // Step properties
        reauthOptions={reauthOptions}
        reauthMethod={reauthMethod}
        credential={credential}
        onReauthMethodChange={setReauthMethod}
        onAuthenticated={setCredential}
        onClose={onClose}
        onSuccess={onSuccess}
      />
    </Dialog>
  );
}

type ReauthenticationMethod = 'passwordless' | 'mfaDevice' | 'otp';
type ReauthenticationOption = {
  value: ReauthenticationMethod;
  label: string;
};

export function createReauthOptions(
  auth2faType: Auth2faType,
  passwordlessEnabled: boolean,
  devices: MfaDevice[]
) {
  const options: ReauthenticationOption[] = [];

  const methodsAllowedByDevices = {};
  for (const d of devices) {
    methodsAllowedByDevices[reauthMethodForDevice(d)] = true;
  }

  if (passwordlessEnabled && 'passwordless' in methodsAllowedByDevices) {
    options.push({ value: 'passwordless', label: 'Passkey' });
  }

  const mfaEnabled = auth2faType === 'on' || auth2faType === 'optional';

  if (
    (auth2faType === 'webauthn' || mfaEnabled) &&
    'mfaDevice' in methodsAllowedByDevices
  ) {
    options.push({ value: 'mfaDevice', label: 'MFA Device' });
  }

  if (
    (auth2faType === 'otp' || mfaEnabled) &&
    'otp' in methodsAllowedByDevices
  ) {
    options.push({ value: 'otp', label: 'Authenticator App' });
  }

  return options;
}

/** Returns the reauthentication method supported by a given device. */
function reauthMethodForDevice(d: MfaDevice): ReauthenticationMethod {
  if (d.usage === 'passwordless') return 'passwordless';
  return d.type === 'totp' ? 'otp' : 'mfaDevice';
}

const wizardFlows = {
  withReauthentication: [ReauthenticateStep, ChangePasswordStep],
  withoutReauthentication: [ChangePasswordStep],
};

interface ReauthenticateStepProps extends StepComponentProps {
  reauthOptions: ReauthenticationOption[];
  reauthMethod: ReauthenticationMethod;
  onReauthMethodChange(method: ReauthenticationMethod): void;
  onAuthenticated(res: Credential): void;
  onClose(): void;
}

export function ReauthenticateStep({
  next,
  refCallback,
  stepIndex,
  flowLength,
  reauthOptions,
  reauthMethod,
  onReauthMethodChange,
  onAuthenticated,
  onClose,
}: ReauthenticateStepProps) {
  const [reauthenticateAttempt, reauthenticate] = useAsync(
    async (m: ReauthenticationMethod) => {
      if (m === 'passwordless' || m === 'mfaDevice') {
        const res = await auth.fetchWebAuthnChallenge({
          scope: MfaChallengeScope.CHANGE_PASSWORD,
          userVerificationRequirement:
            m === 'passwordless' ? 'required' : 'discouraged',
        });
        onAuthenticated(res);
      }
      next();
    }
  );
  const onReauthenticate = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    reauthenticate(reauthMethod);
  };

  return (
    <StepContainer ref={refCallback} data-testid="reauthenticate-step">
      <Box mb={4}>
        <StepHeader
          stepIndex={stepIndex}
          flowLength={flowLength}
          title="Verify Identity"
        />
      </Box>
      {reauthenticateAttempt.status === 'error' && (
        <OutlineDanger>{reauthenticateAttempt.statusText}</OutlineDanger>
      )}
      Verification Method
      <form onSubmit={e => onReauthenticate(e)}>
        <RadioGroup
          name="mfaOption"
          options={reauthOptions}
          value={reauthMethod}
          autoFocus
          flexDirection="row"
          gap={3}
          mb={4}
          onChange={onReauthMethodChange}
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

interface ChangePasswordStepProps extends StepComponentProps {
  credential: Credential;
  reauthMethod: ReauthenticationMethod;
  onClose(): void;
  onSuccess(): void;
}

export function ChangePasswordStep({
  refCallback,
  prev,
  stepIndex,
  flowLength,
  credential,
  reauthMethod,
  onClose,
  onSuccess,
}: ChangePasswordStepProps) {
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [newPassConfirmed, setNewPassConfirmed] = useState('');
  const [authCode, setAuthCode] = useState('');
  const onAuthCodeChanged = (e: React.ChangeEvent<HTMLInputElement>) => {
    setAuthCode(e.target.value);
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
    setAuthCode('');
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
      secondFactorToken: authCode,
      credential,
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
            {reauthMethod === 'otp' && (
              <FieldInput
                label="Authenticator Code"
                labelTip="Enter the code generated by your authenticator app"
                rule={requiredField('Authenticator code is required')}
                inputMode="numeric"
                autoComplete="one-time-code"
                value={authCode}
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
                <ButtonSecondary block={true} onClick={onClose}>
                  Cancel
                </ButtonSecondary>
              ) : (
                <ButtonSecondary block={true} onClick={prev}>
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
