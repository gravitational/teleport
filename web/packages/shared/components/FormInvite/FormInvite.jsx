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
import { Text, Card, ButtonPrimary, Flex, Box } from 'design';
import * as Alerts from 'design/Alert';
import FieldInput from './../FieldInput';
import Validation from './../Validation';
import { isOtp, isU2f } from '../../services/enums';
import TwoFAData from './TwoFaInfo';
import {
  requiredToken,
  requiredPassword,
  requiredConfirmedPassword,
} from './../Validation/rules';

const U2F_ERROR_CODES_URL =
  'https://developers.yubico.com/U2F/Libraries/Client_error_codes.html';

export default function FormInvite(props) {
  const {
    auth2faType,
    onSubmitWithU2f,
    onSubmit,
    attempt,
    user,
    qr,
    title,
    submitBtnText,
  } = props;
  const [password, setPassword] = React.useState('');
  const [passwordConfirmed, setPasswordConfirmed] = React.useState('');
  const [token, setToken] = React.useState('');

  const otpEnabled = isOtp(auth2faType);
  const u2fEnabled = isU2f(auth2faType);
  const secondFactorEnabled = otpEnabled || u2fEnabled;
  const { isProcessing, isFailed, message } = attempt;
  const boxWidth = (secondFactorEnabled ? 720 : 464) + 'px';

  function onBtnClick(e, validator) {
    e.preventDefault();
    if (!validator.validate()) {
      return;
    }

    if (u2fEnabled) {
      onSubmitWithU2f(password);
    } else {
      onSubmit(password, token);
    }
  }

  return (
    <Validation>
      {({ validator }) => (
        <Card as="form" bg="primary.light" my={6} mx="auto" width={boxWidth}>
          <Flex>
            <Box flex="3" p="6">
              <Text typography="h2" mb={3} textAlign="center" color="light">
                {title}
              </Text>
              {isFailed && <ErrorMessage message={message} />}
              <Text typography="h4" breakAll mb={3}>
                {user}
              </Text>
              <FieldInput
                rule={requiredPassword}
                autoFocus
                autoComplete="off"
                label="Password"
                value={password}
                onChange={e => setPassword(e.target.value)}
                type="password"
                placeholder="Password"
              />
              <FieldInput
                rule={requiredConfirmedPassword(password)}
                autoComplete="off"
                label="Confirm Password"
                value={passwordConfirmed}
                onChange={e => setPasswordConfirmed(e.target.value)}
                type="password"
                placeholder="Confirm Password"
              />
              {otpEnabled && (
                <Flex flexDirection="row">
                  <FieldInput
                    label="Two factor token"
                    rule={requiredToken}
                    autoComplete="off"
                    width="50%"
                    value={token}
                    onChange={e => setToken(e.target.value)}
                    placeholder="123 456"
                  />
                </Flex>
              )}
              <ButtonPrimary
                width="100%"
                mt={3}
                disabled={isProcessing}
                size="large"
                onClick={e => onBtnClick(e, validator)}
              >
                {submitBtnText}
              </ButtonPrimary>
              {isProcessing && u2fEnabled && (
                <Text
                  mt="3"
                  typography="paragraph2"
                  width="100%"
                  textAlign="center"
                >
                  Insert your U2F key and press the button on the key
                </Text>
              )}
            </Box>
            {secondFactorEnabled && (
              <Box flex="1" bg="primary.main" p="6" borderTopRightRadius="3" borderBottomRightRadius="3">
                <TwoFAData auth2faType={auth2faType} qr={qr} />
              </Box>
            )}
          </Flex>
        </Card>
      )}
    </Validation>
  );
}

FormInvite.propTypes = {
  submitBtnText: PropTypes.string,
  auth2faType: PropTypes.string,
  onSubmitWithU2f: PropTypes.func.isRequired,
  onSubmit: PropTypes.func.isRequired,
  attempt: PropTypes.object.isRequired,
  user: PropTypes.string.isRequired,
  qr: PropTypes.string,
};

FormInvite.defaultProps = {
  submitBtnText: 'Submit',
};

function ErrorMessage({ message = '' }) {
  // quick fix: check if error text has U2F substring
  const showU2fErrorLink = message.indexOf('U2F') !== -1;
  return (
    <Alerts.Danger>
      <div>
        {message}
        {showU2fErrorLink && (
          <span>
            , click{' '}
            <a target="_blank" href={U2F_ERROR_CODES_URL}>
              here
            </a>{' '}
            to learn more about U2F error codes
          </span>
        )}
      </div>
    </Alerts.Danger>
  );
}
