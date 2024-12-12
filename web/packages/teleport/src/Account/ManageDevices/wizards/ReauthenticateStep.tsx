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
import Flex from 'design/Flex';
import { RadioGroup } from 'design/RadioGroup';
import { StepComponentProps, StepHeader } from 'design/StepSlider';
import React, { FormEvent, useState } from 'react';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import Box from 'design/Box';

import Indicator from 'design/Indicator';

import { useEffect } from 'react';

import useReAuthenticate from 'teleport/components/ReAuthenticate/useReAuthenticate';
import auth, { MfaChallengeScope } from 'teleport/services/auth/auth';

import {
  DeviceType,
  getMfaChallengeOptions,
  MfaOption,
} from 'teleport/services/mfa';

export type ReauthenticateStepProps = StepComponentProps & {
  setPrivilegeToken(token: string): void;
  onClose(): void;
};

export function ReauthenticateStep({
  next,
  refCallback,
  stepIndex,
  flowLength,
  setPrivilegeToken,
  onClose,
}: ReauthenticateStepProps) {
  const [otpCode, setOtpCode] = useState('');

  const {
    getMfaChallenge,
    getChallengeAttempt,
    submitWithMfa,
    submitAttempt,
    clearSubmitAttempt,
  } = useReAuthenticate({
    challengeScope: MfaChallengeScope.MANAGE_DEVICES,
    onMfaResponse: mfaResponse => {
      // TODO(Joerger): v19.0.0
      // Devices can be deleted with an MFA response, so exchanging it for a
      // privilege token adds an unnecessary API call. The device deletion
      // endpoint requires a token, but the new endpoint "DELETE: /webapi/mfa/devices"
      // can be used after v19 backwards compatibly.
      //
      // Adding devices can also be done with an MFA response, but a privilege token
      // gives the user more flexibility in the wizard flow to go back/forward or
      // switch register-device-type without re-prompting MFA. A reusable
      // mfa challenge would be a better fit, or allowing the user to decide device
      // registration type after retrieving the mfa register challenge.
      auth.createPrivilegeToken(mfaResponse).then(setPrivilegeToken);
    },
  });

  const [mfaOption, setMfaOption] = useState<DeviceType>();
  const [mfaOptions, setMfaOptions] = useState<MfaOption[]>();

  useEffect(() => {
    getMfaChallenge().then(getMfaChallengeOptions).then(setMfaOptions);

    // If user has no re-authentication options, continue without re-auth.
    // The user must be registering their first device, which doesn't require re-auth.
    //
    // TODO(Joerger): v19.0.0
    // Registering first device does not require a privilege token anymore, so we
    // could just call next() and return here instead of empty submit w/ setPrivilegeToken.
    // However the existing web register endpoint requires privilege token.
    // We have a new endpoint "/v1/webapi/users/privilege" which does not
    // require token, but can't be used until v19 for backwards compatibility.
    if (mfaOptions.length === 0) {
      submitWithMfa().then(next);
      return;
    }
  });

  // Handle potential mfa challenge error states.
  switch (getChallengeAttempt.status) {
    case 'processing':
      return (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      );
    case 'error':
      return <Alert children={getChallengeAttempt.statusText} />;
    case 'success':
      break;
    default:
      return null;
  }

  const onOtpCodeChanged = (e: React.ChangeEvent<HTMLInputElement>) => {
    setOtpCode(e.target.value);
  };

  const onReauthenticate = (
    e: FormEvent<HTMLFormElement>,
    validator: Validator
  ) => {
    e.preventDefault();
    if (!validator.validate()) return;
    submitWithMfa().then(next);
  };

  return (
    <div ref={refCallback} data-testid="reauthenticate-step">
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
      {mfaOption && <Box mb={2}>Multi-factor type</Box>}
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
                setMfaOption(o as DeviceType);
                clearSubmitAttempt();
              }}
            />
            {mfaOption === 'totp' && (
              <FieldInput
                label="Authenticator Code"
                rule={requiredField('Authenticator code is required')}
                inputMode="numeric"
                autoComplete="one-time-code"
                value={otpCode}
                placeholder="123 456"
                onChange={onOtpCodeChanged}
                readonly={submitAttempt.status === 'processing'}
              />
            )}
            <Flex gap={2}>
              {mfaOption && (
                <ButtonPrimary type="submit" block={true} size="large">
                  Verify my identity
                </ButtonPrimary>
              )}
              <ButtonSecondary
                type="button"
                block={true}
                onClick={onClose}
                size="large"
              >
                Cancel
              </ButtonSecondary>
            </Flex>
          </form>
        )}
      </Validation>
    </div>
  );
}
