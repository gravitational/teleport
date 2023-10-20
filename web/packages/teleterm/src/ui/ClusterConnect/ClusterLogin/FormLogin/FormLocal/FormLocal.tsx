/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
import React, { useState, useMemo } from 'react';
import { Flex, ButtonPrimary, Box } from 'design';

import Validation, { Validator } from 'shared/components/Validation';
import FieldInput from 'shared/components/FieldInput';
import FieldSelect from 'shared/components/FieldSelect';
import {
  requiredToken,
  requiredField,
} from 'shared/components/Validation/rules';
import { useRefAutoFocus } from 'shared/hooks';
import createMfaOptions, { MfaOption } from 'shared/utils/createMfaOptions';

import type { Props } from '../FormLogin';

export const FormLocal = ({
  secondFactor,
  loginAttempt,
  onLogin,
  clearLoginAttempt,
  hasTransitionEnded,
  loggedInUserName,
  autoFocus = false,
}: Props & { hasTransitionEnded?: boolean }) => {
  const [pass, setPass] = useState('');
  const [user, setUser] = useState(loggedInUserName || '');
  const [token, setToken] = useState('');

  const mfaOptions = useMemo(
    () => createMfaOptions({ auth2faType: secondFactor }),
    []
  );

  const [mfaType, setMfaType] = useState(mfaOptions[0]);
  const usernameInputRef = useRefAutoFocus<HTMLInputElement>({
    shouldFocus: hasTransitionEnded && autoFocus && !loggedInUserName,
  });
  const passwordInputRef = useRefAutoFocus<HTMLInputElement>({
    shouldFocus: hasTransitionEnded && autoFocus && !!loggedInUserName,
  });

  function onSetMfaOption(option: MfaOption, validator: Validator) {
    setToken('');
    clearLoginAttempt();
    validator.reset();
    setMfaType(option);
  }

  function onLoginClick(
    e: React.MouseEvent<HTMLButtonElement>,
    validator: Validator
  ) {
    e.preventDefault();
    if (!validator.validate()) {
      return;
    }

    onLogin(user, pass, token, mfaType?.value);
  }

  return (
    <Validation>
      {({ validator }) => (
        <Box as="form" height={secondFactor !== 'off' ? '310px' : 'auto'}>
          <FieldInput
            ref={usernameInputRef}
            rule={requiredField('Username is required')}
            label="Username"
            value={user}
            onChange={e => setUser(e.target.value)}
            placeholder="Username"
            mb={3}
          />
          <FieldInput
            ref={passwordInputRef}
            rule={requiredField('Password is required')}
            label="Password"
            value={pass}
            onChange={e => setPass(e.target.value)}
            type="password"
            placeholder="Password"
            mb={3}
            width="100%"
          />
          {secondFactor !== 'off' && (
            <Flex alignItems="flex-end" mb={4}>
              <FieldSelect
                maxWidth="50%"
                width="100%"
                data-testid="mfa-select"
                label="Two-factor Type"
                value={mfaType}
                options={mfaOptions}
                onChange={opt => onSetMfaOption(opt as MfaOption, validator)}
                mr={3}
                mb={0}
                isDisabled={loginAttempt.status === 'processing'}
                menuIsOpen={true}
              />
              {mfaType.value === 'otp' && (
                <FieldInput
                  width="50%"
                  label="Authenticator Code"
                  rule={requiredToken}
                  autoComplete="one-time-code"
                  inputMode="numeric"
                  value={token}
                  onChange={e => setToken(e.target.value)}
                  placeholder="123 456"
                  mb={0}
                />
              )}
            </Flex>
          )}
          <ButtonPrimary
            width="100%"
            mt={2}
            mb={1}
            type="submit"
            size="large"
            onClick={e => onLoginClick(e, validator)}
            disabled={loginAttempt.status === 'processing'}
          >
            Sign In
          </ButtonPrimary>
        </Box>
      )}
    </Validation>
  );
};
