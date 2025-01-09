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

import { useState } from 'react';

import { Box, Flex } from 'design';
import * as Icon from 'design/Icon';
import { SingleRowBox } from 'design/MultiRowBox';

import cfg from 'teleport/config';
import { MfaDevice } from 'teleport/services/mfa';
import { PasswordState } from 'teleport/services/user';

import { ChangePasswordWizard } from './ChangePasswordWizard';
import { ActionButtonSecondary, Header } from './Header';
import { AuthMethodState, StatePill } from './StatePill';

export interface PasswordBoxProps {
  devices: MfaDevice[];
  passwordState: PasswordState;
  onPasswordChange: () => void;
}

export function PasswordBox({
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
            <Flex gap={2} alignItems="center">
              Password
              <StatePill
                data-testid="password-state-pill"
                state={passwordStateToPillState(passwordState)}
              />
            </Flex>
          }
          icon={<Icon.Password />}
          actions={
            <ActionButtonSecondary onClick={() => setDialogOpen(true)}>
              Change Password
            </ActionButtonSecondary>
          }
        />
      </SingleRowBox>
      {dialogOpen && (
        <ChangePasswordWizard
          hasPasswordless={
            cfg.isPasswordlessEnabled() &&
            devices.some(dev => dev.usage === 'passwordless')
          }
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
