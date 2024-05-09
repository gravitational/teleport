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
import Flex from 'design/Flex';
import Image from 'design/Image';
import Indicator from 'design/Indicator';
import { RadioGroup } from 'design/RadioGroup';
import { StepComponentProps, StepSlider } from 'design/StepSlider';
import Text from 'design/Text';
import React, { useState, useEffect, FormEvent } from 'react';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { useAsync } from 'shared/hooks/useAsync';
import useAttempt from 'shared/hooks/useAttemptNext';
import { Auth2faType } from 'shared/services';
import createMfaOptions, { MfaOption } from 'shared/utils/createMfaOptions';

import Box from 'design/Box';

import { DialogHeader } from 'teleport/Account/DialogHeader';
import useReAuthenticate from 'teleport/components/ReAuthenticate/useReAuthenticate';
import auth from 'teleport/services/auth/auth';
import { DeviceUsage } from 'teleport/services/auth';
import useTeleport from 'teleport/useTeleport';

import { PasskeyBlurb } from '../../../components/Passkeys/PasskeyBlurb';

interface AddAuthDeviceWizardProps {
  /** Indicates usage of the device to be added: MFA or a passkey. */
  usage: DeviceUsage;
  /** MFA type setting, as configured in the cluster's configuration. */
  auth2faType: Auth2faType;
  /**
   * A privilege token that may have been created previously; if present, the
   * reauthentication step will be skipped.
   */
  privilegeToken?: string;
  onClose(): void;
  onSuccess(): void;
}

/** A wizard for adding MFA and passkey devices. */
export function AddAuthDeviceWizard({
  privilegeToken: privilegeTokenProp = '',
  usage,
  auth2faType,
  onClose,
  onSuccess,
}: AddAuthDeviceWizardProps) {
  const reauthRequired = !privilegeTokenProp;
  const [privilegeToken, setPrivilegeToken] = useState(privilegeTokenProp);
  const [credential, setCredential] = useState<Credential>(null);

  const mfaOptions = createMfaOptions({
    auth2faType,
    required: true,
  });

  /** A new MFA device type, irrelevant if usage === 'passkey'. */
  const [newMfaDeviceType, setNewMfaDeviceType] = useState(mfaOptions[0].value);

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
        usage={usage}
        auth2faType={auth2faType}
        privilegeToken={privilegeToken}
        credential={credential}
        newMfaDeviceType={newMfaDeviceType}
        onClose={onClose}
        onAuthenticated={setPrivilegeToken}
        onNewMfaDeviceTypeChange={setNewMfaDeviceType}
        onDeviceCreated={setCredential}
        onSuccess={onSuccess}
      />
    </Dialog>
  );
}

const wizardFlows = {
  withReauthentication: [ReauthenticateStep, CreateDeviceStep, SaveDeviceStep],
  withoutReauthentication: [CreateDeviceStep, SaveDeviceStep],
};

type AddAuthDeviceWizardStepProps = StepComponentProps &
  ReauthenticateStepProps &
  CreateDeviceStepProps &
  SaveKeyStepProps;

interface ReauthenticateStepProps {
  auth2faType: Auth2faType;
  onAuthenticated(privilegeToken: string): void;
  onClose(): void;
}

export function ReauthenticateStep({
  next,
  refCallback,
  stepIndex,
  flowLength,
  auth2faType,
  onClose,
  onAuthenticated: onAuthenticatedProp,
}: AddAuthDeviceWizardStepProps) {
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
      <DialogHeader
        stepIndex={stepIndex}
        flowLength={flowLength}
        title="Verify Identity"
      />
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

interface CreateDeviceStepProps {
  usage: DeviceUsage;
  auth2faType: Auth2faType;
  privilegeToken: string;
  newMfaDeviceType: Auth2faType;
  onNewMfaDeviceTypeChange(o: Auth2faType): void;
  onClose(): void;
  onDeviceCreated(c: Credential): void;
}

export function CreateDeviceStep({
  prev,
  next,
  refCallback,
  stepIndex,
  flowLength,
  usage,
  auth2faType,
  privilegeToken,
  newMfaDeviceType,
  onNewMfaDeviceTypeChange,
  onClose,
  onDeviceCreated,
}: AddAuthDeviceWizardStepProps) {
  const createPasskeyAttempt = useAttempt();
  const onCreate = () => {
    if (usage === 'passwordless' || newMfaDeviceType === 'webauthn') {
      createPasskeyAttempt.run(async () => {
        const credential = await auth.createNewWebAuthnDevice({
          tokenId: privilegeToken,
          deviceUsage: usage,
        });
        onDeviceCreated(credential);
        next();
      });
    } else {
      next();
    }
  };

  return (
    <div ref={refCallback} data-testid="create-step">
      <DialogHeader
        stepIndex={stepIndex}
        flowLength={flowLength}
        title={
          usage === 'passwordless' ? 'Create a Passkey' : 'Create an MFA Method'
        }
      />

      {createPasskeyAttempt.attempt.status === 'failed' && (
        <OutlineDanger>{createPasskeyAttempt.attempt.statusText}</OutlineDanger>
      )}
      {usage === 'passwordless' && (
        <Box mb={4}>
          <PasskeyBlurb />
        </Box>
      )}
      {usage === 'mfa' && (
        <CreateMfaBox
          auth2faType={auth2faType}
          newMfaDeviceType={newMfaDeviceType}
          privilegeToken={privilegeToken}
          onNewMfaDeviceTypeChange={onNewMfaDeviceTypeChange}
        />
      )}
      <Flex gap={2}>
        <ButtonPrimary block={true} onClick={onCreate}>
          {usage === 'passwordless'
            ? 'Create a passkey'
            : 'Create an MFA method'}
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
    </div>
  );
}

function CreateMfaBox({
  auth2faType,
  newMfaDeviceType,
  privilegeToken,
  onNewMfaDeviceTypeChange,
}: {
  auth2faType: Auth2faType;
  newMfaDeviceType: Auth2faType;
  privilegeToken: string;
  onNewMfaDeviceTypeChange(o: Auth2faType): void;
}) {
  const mfaOptions = createMfaOptions({
    auth2faType,
    required: true,
  }).map((o: MfaOption) =>
    // Be more specific about the WebAuthn device type (it's not a passkey).
    o.value === 'webauthn' ? { ...o, label: 'Hardware Device' } : o
  );

  return (
    <>
      Multi-factor type
      <RadioGroup
        name="mfaOption"
        options={mfaOptions}
        value={newMfaDeviceType}
        autoFocus
        flexDirection="row"
        gap={3}
        mb={4}
        onChange={o => {
          onNewMfaDeviceTypeChange(o as Auth2faType);
        }}
      />
      {newMfaDeviceType === 'otp' && (
        <QrCodeBox privilegeToken={privilegeToken} />
      )}
    </>
  );
}

function QrCodeBox({ privilegeToken }: { privilegeToken: string }) {
  const [fetchQrCodeAttempt, fetchQrCode] = useAsync((privilegeToken: string) =>
    auth.createMfaRegistrationChallenge(privilegeToken, 'totp')
  );

  useEffect(() => {
    fetchQrCode(privilegeToken);
  }, []);

  return (
    <Flex
      flexDirection="column"
      borderRadius={8}
      gap={4}
      p={4}
      mb={4}
      bg="interactive.tonal.neutral.0"
    >
      <Flex height="168px" justifyContent="center" alignItems="center">
        {fetchQrCodeAttempt.status === 'error' && (
          <OutlineDanger>
            Could not load the QR code. {fetchQrCodeAttempt.statusText}
          </OutlineDanger>
        )}
        {fetchQrCodeAttempt.status === 'processing' && <Indicator />}
        {fetchQrCodeAttempt.status === 'success' && (
          <Image
            src={`data:image/png;base64,${fetchQrCodeAttempt.data.qrCode}`}
            height="100%"
            style={{
              boxSizing: 'border-box',
              border: '8px solid white',
              borderRadius: '8px',
            }}
          />
        )}
      </Flex>
      <Text typography="body1" textAlign="center" mt={2}>
        Scan the QR Code with any authenticator app.
      </Text>
    </Flex>
  );
}

interface SaveKeyStepProps {
  privilegeToken: string;
  credential: Credential;
  usage: DeviceUsage;
  newMfaDeviceType: Auth2faType;
  onSuccess(): void;
}

export function SaveDeviceStep({
  refCallback,
  prev,
  stepIndex,
  flowLength,
  privilegeToken,
  credential,
  usage,
  newMfaDeviceType,
  onSuccess,
}: AddAuthDeviceWizardStepProps) {
  const ctx = useTeleport();
  const saveAttempt = useAttempt();
  const [deviceName, setDeviceName] = useState('');
  const [authCode, setAuthCode] = useState('');

  const onSave = (e: FormEvent<HTMLFormElement>, validator: Validator) => {
    e.preventDefault();
    if (!validator.validate()) return;
    if (usage === 'passwordless' || newMfaDeviceType === 'webauthn') {
      saveAttempt.run(async () => {
        await ctx.mfaService.saveNewWebAuthnDevice({
          addRequest: {
            tokenId: privilegeToken,
            deviceUsage: usage,
            deviceName,
          },
          credential,
        });
        onSuccess();
      });
    } else {
      saveAttempt.run(async () => {
        await ctx.mfaService.addNewTotpDevice({
          tokenId: privilegeToken,
          secondFactorToken: authCode,
          deviceName,
        });
        onSuccess();
      });
    }
  };

  const onNameChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setDeviceName(e.target.value);
  };

  const onAuthCodeChanged = (e: React.ChangeEvent<HTMLInputElement>) => {
    setAuthCode(e.target.value);
  };

  const label =
    usage === 'passwordless' ? 'Passkey Nickname' : 'MFA Method Name';

  return (
    <div ref={refCallback} data-testid="save-step">
      <DialogHeader
        stepIndex={stepIndex}
        flowLength={flowLength}
        title={
          usage === 'passwordless' ? 'Save the Passkey' : 'Save the MFA method'
        }
      />

      {saveAttempt.attempt.status === 'failed' && (
        <OutlineDanger>{saveAttempt.attempt.statusText}</OutlineDanger>
      )}
      <Validation>
        {({ validator }) => (
          <form onSubmit={e => onSave(e, validator)}>
            <FieldInput
              label={label}
              rule={requiredField(`${label} is required`)}
              value={deviceName}
              placeholder="ex. my-macbookpro"
              autoFocus
              onChange={onNameChange}
              readonly={saveAttempt.attempt.status === 'processing'}
            />

            {usage === 'mfa' && newMfaDeviceType === 'otp' && (
              <FieldInput
                label="Authenticator Code"
                labelTip="Enter the code generated by your authenticator app"
                rule={requiredField('Authenticator code is required')}
                inputMode="numeric"
                autoComplete="one-time-code"
                value={authCode}
                placeholder="123 456"
                onChange={onAuthCodeChanged}
                readonly={saveAttempt.attempt.status === 'processing'}
              />
            )}
            <Flex gap={2}>
              <ButtonPrimary type="submit" block={true}>
                {usage === 'passwordless'
                  ? 'Save the Passkey'
                  : 'Save the MFA method'}
              </ButtonPrimary>
              <ButtonSecondary type="button" block={true} onClick={prev}>
                Back
              </ButtonSecondary>
            </Flex>
          </form>
        )}
      </Validation>
    </div>
  );
}
