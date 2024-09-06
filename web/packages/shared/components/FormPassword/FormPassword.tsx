/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React from 'react';
import { ButtonPrimary, Flex, Box, ButtonSecondary } from 'design';
import * as Alerts from 'design/Alert';

import useAttempt from 'shared/hooks/useAttempt';
import createMfaOptions, { MfaOption } from 'shared/utils/createMfaOptions';

import { Auth2faType, PreferredMfaType } from 'shared/services';

import FieldInput from '../FieldInput';
import FieldSelect from '../FieldSelect';
import Validation, { Validator } from '../Validation';
import {
  requiredToken,
  requiredPassword,
  requiredField,
  requiredConfirmedPassword,
} from '../Validation/rules';

function FormPassword(props: Props) {
  const {
    onChangePassWithWebauthn,
    onChangePass,
    onCancel,
    auth2faType = 'off',
    preferredMfaType,
    showCancel,
  } = props;
  const mfaEnabled = auth2faType !== 'off';

  const [attempt, attemptActions] = useAttempt({});
  const [token, setToken] = React.useState('');
  const [oldPass, setOldPass] = React.useState('');
  const [newPass, setNewPass] = React.useState('');
  const [newPassConfirmed, setNewPassConfirmed] = React.useState('');
  const mfaOptions = React.useMemo<MfaOption[]>(
    () =>
      createMfaOptions({
        auth2faType: auth2faType,
        preferredType: preferredMfaType,
      }),
    []
  );
  const [mfaType, setMfaType] = React.useState(mfaOptions[0]);

  const { isProcessing } = attempt;

  function submit() {
    switch (mfaType?.value) {
      case 'webauthn':
        return onChangePassWithWebauthn(oldPass, newPass);
      default:
        return onChangePass(oldPass, newPass, token);
    }
  }

  function resetForm() {
    setOldPass('');
    setNewPass('');
    setNewPassConfirmed('');
    setToken('');
  }

  function onSubmit(
    e: React.MouseEvent<HTMLButtonElement>,
    validator: Validator
  ) {
    e.preventDefault();
    if (!validator.validate()) {
      return;
    }

    validator.reset();

    attemptActions.start();
    submit()
      .then(() => {
        attemptActions.stop();
        resetForm();
      })
      .catch(err => {
        attemptActions.error(err);
      });
  }

  function onSetMfaOption(option: MfaOption, validator: Validator) {
    setToken('');
    attemptActions.clear();
    validator.reset();
    setMfaType(option);
  }

  return (
    <Validation>
      {({ validator }) => (
        <form>
          <Status attempt={attempt} />
          <FieldInput
            rule={requiredField('Current Password is required')}
            label="Current Password"
            value={oldPass}
            onChange={e => setOldPass(e.target.value)}
            type="password"
            placeholder="Password"
          />
          {mfaEnabled && (
            <Flex alignItems="flex-end" mb={4}>
              <Box width="60%" data-testid="mfa-select">
                <FieldSelect
                  label="Two-factor Type"
                  value={mfaType}
                  options={mfaOptions}
                  onChange={opt => onSetMfaOption(opt as MfaOption, validator)}
                  mr={3}
                  mb={0}
                  isDisabled={isProcessing}
                />
              </Box>
              <Box width="40%">
                {mfaType.value === 'otp' && (
                  <FieldInput
                    label="Authenticator Code"
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    rule={requiredToken}
                    value={token}
                    onChange={e => setToken(e.target.value)}
                    placeholder="123 456"
                    mb={0}
                  />
                )}
              </Box>
            </Flex>
          )}
          <FieldInput
            rule={requiredPassword}
            label="New Password"
            value={newPass}
            onChange={e => setNewPass(e.target.value)}
            type="password"
            placeholder="New Password"
          />
          <FieldInput
            rule={requiredConfirmedPassword(newPass)}
            label="Confirm Password"
            value={newPassConfirmed}
            onChange={e => setNewPassConfirmed(e.target.value)}
            type="password"
            placeholder="Confirm Password"
          />
          <Flex mt={5} gap={5}>
            <ButtonPrimary
              block
              disabled={isProcessing}
              size="large"
              onClick={e => onSubmit(e, validator)}
            >
              Update Password
            </ButtonPrimary>
            {showCancel && (
              <ButtonSecondary
                disabled={isProcessing}
                size="large"
                onClick={onCancel}
              >
                Cancel
              </ButtonSecondary>
            )}
          </Flex>
        </form>
      )}
    </Validation>
  );
}

function Status({ attempt }: StatusProps) {
  if (attempt.isFailed) {
    return <Alerts.Danger>{attempt.message}</Alerts.Danger>;
  }

  if (attempt.isSuccess) {
    return <Alerts.Success>Your password has been changed!</Alerts.Success>;
  }

  return null;
}

type StatusProps = {
  attempt: ReturnType<typeof useAttempt>[0];
};

type Props = {
  auth2faType?: Auth2faType;
  preferredMfaType?: PreferredMfaType;
  showCancel?: boolean;
  onChangePass(oldPass: string, newPass: string, token: string): Promise<any>;
  onChangePassWithWebauthn(oldPass: string, newPass: string): Promise<any>;
  onCancel?(): void;
};

export default FormPassword;
