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

import { Alert, OutlineDanger } from 'design/Alert/Alert';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Dialog from 'design/Dialog';
import Flex from 'design/Flex';
import Image from 'design/Image';
import Indicator from 'design/Indicator';
import { RadioGroup } from 'design/RadioGroup';
import { StepComponentProps, StepSlider, StepHeader } from 'design/StepSlider';
import React, { useState, useEffect, FormEvent } from 'react';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { useAsync } from 'shared/hooks/useAsync';
import useAttempt from 'shared/hooks/useAttemptNext';
import { Auth2faType } from 'shared/services';
import {
  MfaOption,
  createOptionsFromAuth2faType,
} from 'shared/utils/createMfaOptions';

import Box from 'design/Box';

import { P } from 'design/Text/Text';

import auth, { MfaChallengeScope } from 'teleport/services/auth/auth';
import { DeviceUsage, MfaChallengeResponse } from 'teleport/services/auth';
import useTeleport from 'teleport/useTeleport';

import { PasskeyBlurb } from '../../../components/Passkeys/PasskeyBlurb';

import {
  ReauthenticateStep,
  ReauthenticateStepProps,
} from './ReauthenticateStep';

interface AddAuthDeviceWizardProps {
  /** Indicates usage of the device to be added: MFA or a passkey. */
  usage: DeviceUsage;
  /** MFA type setting, as configured in the cluster's configuration. */
  auth2faType: Auth2faType;
  onClose(): void;
  onSuccess(): void;
}

/** A wizard for adding MFA and passkey devices. */
export function AddAuthDeviceWizard({
  usage,
  auth2faType,
  onClose,
  onSuccess,
}: AddAuthDeviceWizardProps) {
  const [mfaResponse, setMfaResponse] = useState<MfaChallengeResponse>(null);
  const [credential, setCredential] = useState<Credential>(null);

  // Choose a new device type from the options available for the given 2fa type.
  // irrelevant if usage === 'passkey'.
  const mfaOptions = createOptionsFromAuth2faType(auth2faType);
  const [newMfaDeviceType, setNewMfaDeviceType] = useState(mfaOptions[0].value);

  // Attempt to get an MFA challenge for an existing device. If the challenge is
  // empty, the user has no existing device (e.g. SSO user) and can register their
  // first device without re-authentication.
  const [challenge, getChallenge] = useAsync(async () => {
    return auth.getChallenge({ scope: MfaChallengeScope.MANAGE_DEVICES });
  });

  useEffect(() => {
    getChallenge();
  }, []);

  const hasChallenge =
    challenge.data?.ssoChallenge ||
    challenge.data?.webauthnPublicKey ||
    challenge.data?.totpChallenge;

  // Handle potential error states first.
  switch (challenge.status) {
    case 'processing':
      return (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      );
    case 'error':
      return <Alert children={challenge.statusText} />;
    case 'success':
      break;
    default:
      return null;
  }

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
          hasChallenge ? 'withReauthentication' : 'withoutReauthentication'
        }
        // Step properties
        mfaChallenge={challenge.data}
        usage={usage}
        mfaOptions={mfaOptions}
        credential={credential}
        newMfaDeviceType={newMfaDeviceType}
        onClose={onClose}
        existingMfaResponse={mfaResponse}
        onMfaResponse={setMfaResponse}
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

export type AddAuthDeviceWizardStepProps = StepComponentProps &
  ReauthenticateStepProps &
  CreateDeviceStepProps &
  SaveKeyStepProps;
interface CreateDeviceStepProps {
  usage: DeviceUsage;
  mfaOptions: MfaOption[];
  existingMfaResponse: MfaChallengeResponse;
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
  existingMfaResponse,
  newMfaDeviceType,
  mfaOptions,
  onNewMfaDeviceTypeChange,
  onClose,
  onDeviceCreated,
}: AddAuthDeviceWizardStepProps) {
  const createPasskeyAttempt = useAttempt();
  const onCreate = () => {
    if (usage === 'passwordless' || newMfaDeviceType === 'webauthn') {
      createPasskeyAttempt.run(async () => {
        const credential = await auth.createNewWebAuthnDevice({
          deviceUsage: usage,
          existingMfaResponse,
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
      <Box mb={4}>
        <StepHeader
          stepIndex={stepIndex}
          flowLength={flowLength}
          title={
            usage === 'passwordless'
              ? 'Create a Passkey'
              : 'Create an MFA Method'
          }
        />
      </Box>

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
          mfaOptions={mfaOptions}
          newMfaDeviceType={newMfaDeviceType}
          existingMfaResponse={existingMfaResponse}
          onNewMfaDeviceTypeChange={onNewMfaDeviceTypeChange}
        />
      )}
      <Flex gap={2}>
        <ButtonPrimary block={true} size="large" onClick={onCreate}>
          {usage === 'passwordless'
            ? 'Create a passkey'
            : 'Create an MFA method'}
        </ButtonPrimary>
        {stepIndex === 0 ? (
          <ButtonSecondary block={true} size="large" onClick={onClose}>
            Cancel
          </ButtonSecondary>
        ) : (
          <ButtonSecondary block={true} size="large" onClick={prev}>
            Back
          </ButtonSecondary>
        )}
      </Flex>
    </div>
  );
}

function CreateMfaBox({
  mfaOptions,
  newMfaDeviceType,
  existingMfaResponse,
  onNewMfaDeviceTypeChange,
}: {
  mfaOptions: MfaOption[];
  newMfaDeviceType: Auth2faType;
  existingMfaResponse: MfaChallengeResponse;
  onNewMfaDeviceTypeChange(o: Auth2faType): void;
}) {
  // Be more specific about the WebAuthn device type (it's not a passkey).
  mfaOptions = mfaOptions.map((o: MfaOption) =>
    o.value === 'webauthn' ? { ...o, label: 'Hardware Device' } : o
  );

  return (
    <>
      <Box mb={2}>Multi-factor type</Box>
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
        <QrCodeBox existingMfaResponse={existingMfaResponse} />
      )}
    </>
  );
}

function QrCodeBox({
  existingMfaResponse,
}: {
  existingMfaResponse: MfaChallengeResponse;
}) {
  const [fetchQrCodeAttempt, fetchQrCode] = useAsync(
    (existingMfaResponse: MfaChallengeResponse) =>
      auth.createMfaRegistrationChallenge({
        deviceType: 'totp',
        existingMfaResponse: existingMfaResponse,
      })
  );

  useEffect(() => {
    fetchQrCode(existingMfaResponse);
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
      <P textAlign="center" mt={2}>
        Scan the QR Code with any authenticator app.
      </P>
    </Flex>
  );
}

interface SaveKeyStepProps {
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
  existingMfaResponse: existingMfaResponse,
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
            deviceUsage: usage,
            deviceName,
            existingMfaResponse,
          },
          credential,
        });
        onSuccess();
      });
    } else {
      saveAttempt.run(async () => {
        await ctx.mfaService.addNewTotpDevice({
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
      <Box mb={4}>
        <StepHeader
          stepIndex={stepIndex}
          flowLength={flowLength}
          title={
            usage === 'passwordless'
              ? 'Save the Passkey'
              : 'Save the MFA method'
          }
        />
      </Box>

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
                helperText="Enter the code generated by your authenticator app"
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
              <ButtonPrimary type="submit" block={true} size="large">
                {usage === 'passwordless'
                  ? 'Save the Passkey'
                  : 'Save the MFA method'}
              </ButtonPrimary>
              <ButtonSecondary
                type="button"
                block={true}
                size="large"
                onClick={prev}
              >
                Back
              </ButtonSecondary>
            </Flex>
          </form>
        )}
      </Validation>
    </div>
  );
}
