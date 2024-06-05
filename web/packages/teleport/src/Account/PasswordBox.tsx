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

import { Box, Flex } from 'design';
import { SingleRowBox } from 'design/MultiRowBox';
import React, { useState } from 'react';

import * as Icon from 'design/Icon';

import cfg from 'teleport/config';

import { MfaDevice } from 'teleport/services/mfa';

import { PasswordState } from 'teleport/services/user';

import { ActionButtonSecondary, Header } from './Header';
import { ChangePasswordWizard } from './ChangePasswordWizard';
import { StatePill, AuthMethodState } from './StatePill';

export interface PasswordBoxProps {
  changeDisabled: boolean;
  devices: MfaDevice[];
  passwordState: PasswordState;
  onPasswordChange: () => void;
}

export function PasswordBox({
  changeDisabled,
  devices,
  passwordState,
  onPasswordChange,
}: PasswordBoxProps) {
  const [dialogOpen, setDialogOpen] = useState(false);

  const onSuccess = () => {
    setDialogOpen(false);
    onPasswordChange();
  };

  return (
    <Box>
      <SingleRowBox>
        <Header
          title={
            <Flex gap={2}>
              Password
              <StatePill
                data-testid="password-state-pill"
                state={passwordStateToPillState(passwordState)}
              />
            </Flex>
          }
          icon={<Icon.Password />}
          actions={
            <ActionButtonSecondary
              disabled={changeDisabled}
              onClick={() => setDialogOpen(true)}
            >
              Change Password
            </ActionButtonSecondary>
          }
        />
      </SingleRowBox>
      {dialogOpen && (
        <ChangePasswordWizard
          auth2faType={cfg.getAuth2faType()}
          passwordlessEnabled={cfg.isPasswordlessEnabled()}
          devices={devices}
          onClose={() => setDialogOpen(false)}
          onSuccess={onSuccess}
        />
      )}
    </Box>
  );
}

function passwordStateToPillState(
  state: PasswordState
): AuthMethodState | undefined {
  switch (state) {
    case PasswordState.PASSWORD_STATE_SET:
      return 'active';
    case PasswordState.PASSWORD_STATE_UNSET:
      return 'inactive';
    default:
      state satisfies never | PasswordState.PASSWORD_STATE_UNSPECIFIED;
      return undefined;
  }
}
