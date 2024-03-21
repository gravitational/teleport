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

import { Box } from 'design';
import { SingleRowBox } from 'design/MultiRowBox';
import React, { useState } from 'react';

import * as Icon from 'design/Icon';

import cfg from 'teleport/config';

import { MfaDevice } from 'teleport/services/mfa';

import { ActionButton, Header } from './Header';
import { ChangePasswordWizard } from './ChangePasswordWizard';

export interface PasswordBoxProps {
  changeDisabled: boolean;
  devices: MfaDevice[];
  onPasswordChange: () => void;
}

export function PasswordBox({
  changeDisabled,
  devices,
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
          title="Password"
          icon={<Icon.Password />}
          actions={
            <ActionButton
              disabled={changeDisabled}
              onClick={() => setDialogOpen(true)}
            >
              Change Password
            </ActionButton>
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
