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
import PropTypes from 'prop-types';
import { Card, ButtonPrimary } from 'design';
import * as Alerts from 'design/Alert';
import useAttempt from './../../hooks/useAttempt';
import FieldInput from './../FieldInput';
import Validation from '../Validation';
import {
  requiredToken,
  requiredPassword,
  requiredField,
  requiredConfirmedPassword,
} from './../Validation/rules';
import { isOtp, isU2f } from './../../services/enums';

function FormPassword(props) {
  const { onChangePassWithU2f, onChangePass, auth2faType } = props;
  const [attempt, attemptActions] = useAttempt();
  const [token, setToken] = React.useState('');
  const [oldPass, setOldPass] = React.useState('');
  const [newPass, setNewPass] = React.useState('');
  const [newPassConfirmed, setNewPassConfirmed] = React.useState('');

  const otpEnabled = isOtp(auth2faType);
  const u2fEnabled = isU2f(auth2faType);
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

  function onSubmit(e, validator) {
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
          <Status u2f={u2fEnabled} attempt={attempt} />
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

FormPassword.propTypes = {
  onChangePass: PropTypes.func.isRequired,
  onChangePassWithU2f: PropTypes.func.isRequired,

  /**
   * auth2faType defines login type.
   * eg: u2f, otp, off (disabled).
   *
   * enums are defined in shared/services/enums.js
   */
  auth2faType: PropTypes.string,
};

function Status({ attempt, u2f }) {
  const { isProcessing, isFailed, message, isSuccess } = attempt;

  if (isFailed) {
    return <Alerts.Danger>{message}</Alerts.Danger>;
  }

  if (isSuccess) {
    return <Alerts.Success>Your password has been changed!</Alerts.Success>;
  }

  const waitForU2fKeyResponse = isProcessing && u2f;
  if (waitForU2fKeyResponse) {
    return (
      <Alerts.Info>
        Insert your U2F key and press the button on the key
      </Alerts.Info>
    );
  }

  return null;
}

export default FormPassword;
