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

  const isProcessing = loginAttempt.status === 'processing';

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
            disabled={isProcessing}
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
            disabled={isProcessing}
          />
          {secondFactor !== 'off' && (
            <Flex alignItems="flex-end" mb={4}>
              <FieldSelect
                maxWidth="60%"
                width="100%"
                data-testid="mfa-select"
                label="Two-factor Type"
                value={mfaType}
                options={mfaOptions}
                onChange={opt => onSetMfaOption(opt as MfaOption, validator)}
                mr={3}
                mb={0}
                isDisabled={isProcessing}
                menuIsOpen={true}
              />
              {mfaType.value === 'otp' && (
                <FieldInput
                  width="40%"
                  label="Authenticator Code"
                  rule={requiredToken}
                  autoComplete="one-time-code"
                  inputMode="numeric"
                  value={token}
                  onChange={e => setToken(e.target.value)}
                  placeholder="123 456"
                  mb={0}
                  disabled={isProcessing}
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
            disabled={isProcessing}
          >
            Sign In
          </ButtonPrimary>
        </Box>
      )}
    </Validation>
  );
};
