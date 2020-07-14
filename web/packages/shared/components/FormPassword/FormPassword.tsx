/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { Card, ButtonPrimary } from 'design';
import * as Alerts from 'design/Alert';
import useAttempt from 'shared/hooks/useAttempt';
import FieldInput from '../FieldInput';
import Validation, { Validator } from '../Validation';
import {
  requiredToken,
  requiredPassword,
  requiredField,
  requiredConfirmedPassword,
} from '../Validation/rules';
import { Auth2faType } from 'shared/services';

function FormPassword(props: Props) {
  const { onChangePassWithU2f, onChangePass, auth2faType = 'off' } = props;
  const [attempt, attemptActions] = useAttempt({});
  const [token, setToken] = React.useState('');
  const [oldPass, setOldPass] = React.useState('');
  const [newPass, setNewPass] = React.useState('');
  const [newPassConfirmed, setNewPassConfirmed] = React.useState('');

  const otpEnabled = auth2faType === 'otp';
  const u2fEnabled = auth2faType === 'u2f';
  const { isProcessing } = attempt;

  function submit() {
    if (u2fEnabled) {
      return onChangePassWithU2f(oldPass, newPass);
    }

    return onChangePass(oldPass, newPass, token);
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

  return (
    <Validation>
      {({ validator }) => (
        <Card as="form" bg="primary.light" width="456px" p="6">
          <Status isU2F={u2fEnabled} attempt={attempt} />
          <FieldInput
            rule={requiredField('Current Password is required')}
            label="Current Password"
            value={oldPass}
            onChange={e => setOldPass(e.target.value)}
            type="password"
            placeholder="Password"
          />
          {otpEnabled && (
            <FieldInput
              label="2nd factor token"
              rule={requiredToken}
              width="50%"
              value={token}
              onChange={e => setToken(e.target.value)}
              type="text"
              placeholder="OTP Token"
            />
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
          <ButtonPrimary
            block
            disabled={isProcessing}
            size="large"
            onClick={e => onSubmit(e, validator)}
            mt={5}
          >
            Update Password
          </ButtonPrimary>
        </Card>
      )}
    </Validation>
  );
}

function Status({ attempt, isU2F }: StatusProps) {
  if (attempt.isFailed) {
    return <Alerts.Danger>{attempt.message}</Alerts.Danger>;
  }

  if (attempt.isSuccess) {
    return <Alerts.Success>Your password has been changed!</Alerts.Success>;
  }

  const waitForU2fKeyResponse = attempt.isProcessing && isU2F;
  if (waitForU2fKeyResponse) {
    return (
      <Alerts.Info>
        Insert your U2F key and press the button on the key
      </Alerts.Info>
    );
  }

  return null;
}

type StatusProps = {
  attempt: ReturnType<typeof useAttempt>[0];
  isU2F: boolean;
};

type Props = {
  auth2faType?: Auth2faType;
  onChangePass(oldPass: string, newPass: string, token: string): Promise<any>;
  onChangePassWithU2f(oldPass: string, newPass: string): Promise<any>;
};

export default FormPassword;
