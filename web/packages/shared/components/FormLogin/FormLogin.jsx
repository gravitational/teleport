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
import { Card, Box, Text, Flex, ButtonLink, ButtonPrimary } from 'design';
import * as Alerts from 'design/Alert';
import { isU2f, isOtp } from './../../services/enums';
import SsoButtonList from './SsoButtons';
import Validation from '../Validation';
import FieldInput from '../FieldInput';
import { requiredToken, requiredField } from '../Validation/rules';

export default function LoginForm(props) {
  const {
    title,
    attempt,
    auth2faType,
    onLoginWithU2f,
    onLogin,
    onLoginWithSso,
    authProviders,
  } = props;

  const [pass, setPass] = React.useState('');
  const [user, setUser] = React.useState('');
  const [token, setToken] = React.useState('');

  const u2fEnabled = isU2f(auth2faType);
  const otpEnabled = isOtp(auth2faType);
  const ssoEnabled = authProviders && authProviders.length > 0;
  const { isFailed, isProcessing, message } = attempt;

  function onLoginClick(e, validator) {
    e.preventDefault();
    if (!validator.validate()) {
      return;
    }

    if (u2fEnabled) {
      onLoginWithU2f(user, pass);
    } else {
      onLogin(user, pass, token);
    }
  }

  return (
    <Validation>
      {({ validator }) => (
        <Card as="form" bg="primary.light" my="5" mx="auto" width="456px">
          <Box p="6">
            <Text typography="h2" mb={3} textAlign="center" color="light">
              {title}
            </Text>
            {isFailed && <Alerts.Danger> {message} </Alerts.Danger>}
            <FieldInput
              rule={requiredField('Username is required')}
              label="Username"
              autoFocus
              value={user}
              onChange={e => setUser(e.target.value)}
              placeholder="User name"
            />
            <FieldInput
              rule={requiredField('Password is required')}
              label="Password"
              value={pass}
              onChange={e => setPass(e.target.value)}
              type="password"
              placeholder="Password"
            />
            {otpEnabled && (
              <Flex flexDirection="row">
                <FieldInput
                  label="Two factor token"
                  rule={requiredToken}
                  autoComplete="off"
                  width="200px"
                  value={token}
                  onChange={e => setToken(e.target.value)}
                  placeholder="123 456"
                />
                <ButtonLink
                  width="100%"
                  kind="secondary"
                  target="_blank"
                  size="small"
                  href="https://support.google.com/accounts/answer/1066447?co=GENIE.Platform%3DiOS&hl=en&oco=0"
                  rel="noreferrer"
                >
                  Download Google Authenticator
                </ButtonLink>
              </Flex>
            )}
            <ButtonPrimary
              width="100%"
              my="3"
              type="submit"
              size="large"
              onClick={e => onLoginClick(e, validator)}
              disabled={isProcessing}
            >
              LOGIN
            </ButtonPrimary>
            {isProcessing && u2fEnabled && (
              <Text typography="paragraph2" width="100%" textAlign="center">
                Insert your U2F key and press the button on the key
              </Text>
            )}
          </Box>
          {ssoEnabled && (
            <Box
              as="footer"
              bg="primary.main"
              borderBottomLeftRadius="3"
              borderBottomRightRadius="3"
            >
              <SsoButtonList
                prefixText="Login with "
                isDisabled={isProcessing}
                providers={authProviders}
                onClick={onLoginWithSso}
              />
            </Box>
          )}
        </Card>
      )}
    </Validation>
  );
}

LoginForm.propTypes = {
  auth2faType: PropTypes.string,
  onLoginWithU2f: PropTypes.func.isRequired,
  onLogin: PropTypes.func.isRequired,
  onLoginWithSso: PropTypes.func.isRequired,
  attempt: PropTypes.object.isRequired,
};
