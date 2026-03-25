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

import React, { useState } from 'react';

import { ButtonPrimary, Flex } from 'design';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { useRefAutoFocus } from 'shared/hooks';

import { LinearProgress } from 'teleterm/ui/components/LinearProgress';

import type { Props } from '../FormLogin';

export const FormLocal = ({
  loginAttempt,
  onLogin,
  hasTransitionEnded,
  loggedInUserName,
  autoFocus = false,
}: Props & { hasTransitionEnded?: boolean }) => {
  const [pass, setPass] = useState('');
  const [user, setUser] = useState(loggedInUserName || '');

  // loginAttempt.status needs to be checked here so that auto focus is automatically managed when
  // the form reverts from a processing state to an error state.
  const isAutoFocusAllowed =
    autoFocus && hasTransitionEnded && loginAttempt.status !== 'processing';
  // useRefAutoFocus is used instead of just plain autoFocus because of some weird focus issues
  // stemming from, most probably, using <StepSlider> within a modal. autoFocus generally works
  // within modals. We've never documented why we use this hook so that knowledge is lost in time.
  const usernameInputRef = useRefAutoFocus<HTMLInputElement>({
    shouldFocus: isAutoFocusAllowed && !loggedInUserName,
  });
  const passwordInputRef = useRefAutoFocus<HTMLInputElement>({
    shouldFocus: isAutoFocusAllowed && !!loggedInUserName,
  });

  function onLoginClick(
    e: React.MouseEvent<HTMLButtonElement>,
    validator: Validator
  ) {
    e.preventDefault();
    if (!validator.validate()) {
      return;
    }

    onLogin(user, pass);
  }

  const isProcessing = loginAttempt.status === 'processing';

  return (
    <Validation>
      {({ validator }) => (
        <Flex as="form" flexDirection="column" gap={3}>
          <FieldInput
            ref={usernameInputRef}
            rule={requiredField('Username is required')}
            label="Username"
            value={user}
            onChange={e => setUser(e.target.value)}
            placeholder="Username"
            mb={0}
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
            mb={0}
            width="100%"
            disabled={isProcessing}
          />
          <Flex flexDirection="column" gap={2}>
            <LinearProgress absolute={false} hidden={!isProcessing} />
            <ButtonPrimary
              width="100%"
              mb={0}
              type="submit"
              size="large"
              onClick={e => onLoginClick(e, validator)}
              disabled={isProcessing}
            >
              Sign In
            </ButtonPrimary>
          </Flex>
        </Flex>
      )}
    </Validation>
  );
};
