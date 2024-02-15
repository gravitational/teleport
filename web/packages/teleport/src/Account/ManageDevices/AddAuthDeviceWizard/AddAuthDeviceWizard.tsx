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

import { OutlineDanger } from 'design/Alert/Alert';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Dialog from 'design/Dialog';
import { SingleRowBox } from 'design/MultiRowBox';
import { RadioGroup } from 'design/RadioGroup';
import { StepComponentProps, StepSlider } from 'design/StepSlider';
import Text from 'design/Text';
import React, { useState, FormEvent } from 'react';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import useAttempt from 'shared/hooks/useAttemptNext';
import { Auth2faType } from 'shared/services';
import createMfaOptions from 'shared/utils/createMfaOptions';

import useReAuthenticate from 'teleport/components/ReAuthenticate/useReAuthenticate';
import { MfaChallengeScope } from 'teleport/services/auth/auth';
import useTeleport from 'teleport/useTeleport';

interface AddAuthDeviceWizardProps {
  privilegeToken?: string;
  onClose(): void;
  onSuccess(): void;
}

/**
 * A wizard for adding MFA and passkey devices. Currently only supports
 * passkeys.
 */
export function AddAuthDeviceWizard({
  privilegeToken: privilegeTokenProp = '',
  onClose,
  onSuccess,
}: AddAuthDeviceWizardProps) {
  const reauthRequired = !privilegeTokenProp;
  const [privilegeToken, setPrivilegeToken] = useState(privilegeTokenProp);
  const [credential, setCredential] = useState<Credential>(null);

  return (
    <Dialog
      open={true}
      disableEscapeKeyDown={false}
      dialogCss={() => ({ width: '650px' })}
      onClose={onClose}
    >
      <StepSlider
        flows={wizardFlows}
        currFlow={
          reauthRequired ? 'withReauthentication' : 'withoutReauthentication'
        }
        // Step properties
        privilegeToken={privilegeToken}
        credential={credential}
        onClose={onClose}
        onSuccess={onSuccess}
        onAuthenticated={setPrivilegeToken}
        onPasskeyCreated={setCredential}
      />
    </Dialog>
  );
}

const wizardFlows = {
  withReauthentication: [
    ReauthenticateStep,
    CreatePasskeyStep,
    SavePasskeyStep,
  ],
  withoutReauthentication: [CreatePasskeyStep, SavePasskeyStep],
};

interface ReauthenticateStepProps extends StepComponentProps {
  onAuthenticated(privilegeToken: string): void;
  onClose(): void;
}

function ReauthenticateStep({
  next,
  refCallback,
  stepIndex,
  flowLength,
  onClose,
  onAuthenticated: onAuthenticatedProp,
}: ReauthenticateStepProps) {
  const challengeScope = MfaChallengeScope.MANAGE_DEVICES;
  const onAuthenticated = (privilegeToken: string) => {
    onAuthenticatedProp(privilegeToken);
    next();
  };
  const {
    attempt,
    auth2faType,
    preferredMfaType,
    clearAttempt,
    submitWithTotp,
    submitWithWebauthn,
  } = useReAuthenticate({
    onAuthenticated,
    challengeScope,
  });
  const mfaOptions = createMfaOptions({
    auth2faType,
    preferredType: preferredMfaType,
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
      submitWithWebauthn(challengeScope);
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
      <Text typography="body2">
        Step {stepIndex + 1} of {flowLength}
      </Text>
      <Text typography="h4">Verify Identity</Text>
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
              flexDirection="row"
              gap={3}
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
                autoFocus
                onChange={onAuthCodeChanged}
                readonly={attempt.status === 'processing'}
              />
            )}
            <ButtonPrimary type="submit">Verify my identity</ButtonPrimary>
            <ButtonSecondary onClick={onClose}>Cancel</ButtonSecondary>
          </form>
        )}
      </Validation>
    </div>
  );
}

interface CreatePasskeyStepProps extends StepComponentProps {
  privilegeToken: string;
  onClose(): void;
  onPasskeyCreated(c: Credential): void;
}

function CreatePasskeyStep({
  prev,
  next,
  refCallback,
  stepIndex,
  flowLength,
  privilegeToken,
  onClose,
  onPasskeyCreated,
}: CreatePasskeyStepProps) {
  const ctx = useTeleport();
  const createPasskeyAttempt = useAttempt();

  const onCreate = () => {
    createPasskeyAttempt.run(async () => {
      const credential = await ctx.mfaService.createNewWebAuthnDevice({
        tokenId: privilegeToken,
        deviceUsage: 'passwordless',
      });
      onPasskeyCreated(credential);
      next();
    });
  };

  return (
    <div ref={refCallback} data-testid="create-step">
      <Text typography="body2">
        Step {stepIndex + 1} of {flowLength}
      </Text>
      <Text typography="h4">Create a Passkey</Text>
      {createPasskeyAttempt.attempt.status === 'failed' && (
        <OutlineDanger>{createPasskeyAttempt.attempt.statusText}</OutlineDanger>
      )}
      <PasskeyBlurb />
      <ButtonPrimary onClick={onCreate}>Create a passkey</ButtonPrimary>
      {stepIndex === 0 ? (
        <ButtonSecondary onClick={onClose}>Cancel</ButtonSecondary>
      ) : (
        <ButtonSecondary onClick={prev}>Back</ButtonSecondary>
      )}
    </div>
  );
}

interface SaveKeyStepProps extends StepComponentProps {
  privilegeToken: string;
  credential: Credential;
  onSuccess(): void;
}

function SavePasskeyStep({
  refCallback,
  prev,
  stepIndex,
  flowLength,
  privilegeToken,
  credential,
  onSuccess,
}: SaveKeyStepProps) {
  const ctx = useTeleport();
  const saveAttempt = useAttempt();
  const [deviceName, setDeviceName] = useState('');

  const onSave = (e: FormEvent<HTMLFormElement>, validator: Validator) => {
    e.preventDefault();
    if (!validator.validate()) return;
    saveAttempt.run(async () => {
      await ctx.mfaService.saveNewWebAuthnDevice({
        addRequest: {
          tokenId: privilegeToken,
          deviceUsage: 'passwordless',
          deviceName,
        },
        credential,
      });
      onSuccess();
    });
  };

  const onNameChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setDeviceName(e.target.value);
  };

  return (
    <div ref={refCallback} data-testid="save-step">
      <Text typography="body2">
        Step {stepIndex + 1} of {flowLength}
      </Text>
      <Text typography="h4">Save the Passkey</Text>
      {saveAttempt.attempt.status === 'failed' && (
        <OutlineDanger>{saveAttempt.attempt.statusText}</OutlineDanger>
      )}
      <Validation>
        {({ validator }) => (
          <form onSubmit={e => onSave(e, validator)}>
            <FieldInput
              label="Passkey Nickname"
              rule={requiredField('Passkey nickname is required')}
              value={deviceName}
              placeholder="ex. my-macbookpro"
              autoFocus
              onChange={onNameChange}
              readonly={saveAttempt.attempt.status === 'processing'}
            />
            <ButtonPrimary type="submit">Save the Passkey</ButtonPrimary>
            <ButtonSecondary onClick={prev}>Back</ButtonSecondary>
          </form>
        )}
      </Validation>
    </div>
  );
}

function PasskeyBlurb() {
  return (
    <SingleRowBox>
      <p>
        Teleport supports passkeys, a password replacement that validates your
        identity using touch, facial recognition, a device password, or a PIN.
      </p>
      <p>
        Passkeys can be used to sign in as a simple and secure alternative to
        your password and multi-factor credentials.
      </p>
    </SingleRowBox>
  );
}
