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

import React, { useState } from 'react';
import styled from 'styled-components';
import { Text, Flex, ButtonLink, ButtonPrimary, Box } from 'design';
import * as Alerts from 'design/Alert';
import Validation, { Validator } from 'shared/components/Validation';
import FieldInput from 'shared/components/FieldInput';
import FieldSelect from 'shared/components/FieldSelect';
import {
  requiredToken,
  requiredField,
} from 'shared/components/Validation/rules';
import { Attempt } from 'teleterm/ui/useAsync';
import createMfaOptions, { MfaOption } from 'shared/utils/createMfaOptions';
import * as types from 'teleterm/ui/services/clusters/types';
import SSOButtonList from './SsoButtons';
import PromptHardwareKey from './PromptHardwareKey';
import PromptSsoStatus from './PromptSsoStatus';

export default function LoginForm(props: Props) {
  const {
    title,
    loginAttempt,
    preferredMfa,
    onAbort,
    onLogin,
    onLoginWithSso,
    authProviders,
    auth2faType,
    isLocalAuthEnabled = true,
    shouldPromptSsoStatus,
    shouldPromptHardwareKey,
  } = props;

  const isProcessing = loginAttempt.status === 'processing';
  const ssoEnabled = authProviders && authProviders.length > 0;
  const mfaOptions = createMfaOptions({
    auth2faType,
    preferredType: preferredMfa,
  });
  const [mfaType, setMfaType] = useState<MfaOption>(mfaOptions[0]);
  const [pass, setPass] = useState('');
  const [user, setUser] = useState('');
  const [token, setToken] = useState('');

  const [isExpanded, toggleExpander] = useState(
    !(isLocalAuthEnabled && ssoEnabled)
  );

  function handleLocalLoginClick() {
    onLogin(user, pass, token, mfaType?.value);
  }

  function handleChangeMfaOption(option: MfaOption, validator: Validator) {
    setToken('');
    validator.reset();
    setMfaType(option);
  }

  if (!ssoEnabled && !isLocalAuthEnabled) {
    return <LoginDisabled title={title} />;
  }

  if (shouldPromptHardwareKey) {
    return <PromptHardwareKey onCancel={onAbort} />;
  }

  if (shouldPromptSsoStatus) {
    return <PromptSsoStatus onCancel={onAbort} />;
  }

  return (
    <Validation>
      {({ validator }) => (
        <>
          {loginAttempt.status === 'error' && (
            <Alerts.Danger>{loginAttempt.statusText}</Alerts.Danger>
          )}
          {ssoEnabled && (
            <SSOButtonList
              prefixText="Login with"
              isDisabled={loginAttempt.status === 'processing'}
              providers={authProviders}
              onClick={onLoginWithSso}
            />
          )}
          {ssoEnabled && isLocalAuthEnabled && (
            <Flex
              alignItems="center"
              justifyContent="center"
              style={{ position: 'relative' }}
              flexDirection="column"
              mb={1}
            >
              <StyledOr>Or</StyledOr>
            </Flex>
          )}
          {ssoEnabled && isLocalAuthEnabled && !isExpanded && (
            <FlexBordered flexDirection="row">
              <ButtonLink autoFocus onClick={() => toggleExpander(!isExpanded)}>
                Sign in with your Username and Password
              </ButtonLink>
            </FlexBordered>
          )}
          {isLocalAuthEnabled && isExpanded && (
            <FlexBordered as="form" onSubmit={preventDefault}>
              <FieldInput
                rule={requiredField('Username is required')}
                label="Username"
                autoFocus
                value={user}
                onChange={e => setUser(e.target.value)}
                placeholder="Username"
              />
              <Box mb={4}>
                <FieldInput
                  rule={requiredField('Password is required')}
                  label="Password"
                  value={pass}
                  onChange={e => setPass(e.target.value)}
                  type="password"
                  placeholder="Password"
                  mb={0}
                  width="100%"
                />
              </Box>
              {auth2faType !== 'off' && (
                <Box mb={4}>
                  <Flex alignItems="flex-end">
                    <FieldSelect
                      menuPosition="fixed"
                      maxWidth="50%"
                      width="100%"
                      data-testid="mfa-select"
                      label="Two-factor type"
                      value={mfaType}
                      options={mfaOptions}
                      onChange={opt =>
                        handleChangeMfaOption(opt as MfaOption, validator)
                      }
                      mr={3}
                      mb={0}
                      isDisabled={isProcessing}
                    />
                    {mfaType.value === 'otp' && (
                      <FieldInput
                        width="50%"
                        label="Authenticator code"
                        rule={requiredToken}
                        inputMode="numeric"
                        autoComplete="one-time-code"
                        value={token}
                        onChange={e => setToken(e.target.value)}
                        placeholder="123 456"
                        mb={0}
                      />
                    )}
                    {mfaType.value === 'u2f' && isProcessing && (
                      <Text typography="body2" mb={1}>
                        Insert your hardware key and press the button on the
                        key.
                      </Text>
                    )}
                  </Flex>
                </Box>
              )}
              <ButtonPrimary
                width="100%"
                mt={3}
                type="submit"
                size="large"
                onClick={() => validator.validate() && handleLocalLoginClick()}
                disabled={isProcessing}
              >
                LOGIN
              </ButtonPrimary>
            </FlexBordered>
          )}
        </>
      )}
    </Validation>
  );
}

const FlexBordered = props => (
  <Flex justifyContent="center" flexDirection="column" {...props} />
);

const LoginDisabled = ({ title = '' }) => (
  <>
    <Alerts.Danger my={5}>Login has not been enabled for {title}</Alerts.Danger>
    <Text mb={2} typography="paragraph2" width="100%">
      The ability to login has not been enabled. Please contact your system
      administrator for more information.
    </Text>
  </>
);

const StyledOr = styled.div`
  background: ${props => props.theme.colors.primary.light};
  display: flex;
  align-items: center;
  font-size: 10px;
  height: 32px;
  width: 32px;
  justify-content: center;
  border-radius: 50%;
`;

type LoginAttempt = Attempt<void>;

type Props = {
  shouldPromptSsoStatus: boolean;
  shouldPromptHardwareKey: boolean;
  loginAttempt: LoginAttempt;
  title?: string;
  isLocalAuthEnabled?: boolean;
  preferredMfa: types.PreferredMfaType;
  auth2faType?: types.Auth2faType;
  authProviders: types.AuthProvider[];
  onAbort(): void;
  onLoginWithSso(provider: types.AuthProvider): void;
  onLogin(
    username: string,
    password: string,
    token: string,
    auth2fa: types.Auth2faType
  ): void;
};

const preventDefault = (e: React.SyntheticEvent) => {
  e.stopPropagation();
  e.preventDefault();
};
