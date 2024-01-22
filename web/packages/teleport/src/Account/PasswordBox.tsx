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
import Dialog, {
  DialogContent,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import FormPassword from 'shared/components/FormPassword';

import { ActionButton, Header } from './Header';
import useChangePassword from './ChangePassword/useChangePassword';

export interface PasswordBoxProps {
  changeDisabled: boolean;
  onPasswordChange: () => void;
}

export function PasswordBox({
  changeDisabled,
  onPasswordChange,
}: PasswordBoxProps) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const {
    changePassword,
    changePasswordWithWebauthn,
    preferredMfaType,
    auth2faType,
  } = useChangePassword();

  async function onChangePass(
    oldPass: string,
    newPass: string,
    token: string
  ): Promise<void> {
    await changePassword(oldPass, newPass, token);
    setDialogOpen(false);
    onPasswordChange();
  }

  async function onChangePassWithWebauthn(
    oldPass: string,
    newPass: string
  ): Promise<void> {
    await changePasswordWithWebauthn(oldPass, newPass);
    setDialogOpen(false);
    onPasswordChange();
  }

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
      <Dialog
        open={dialogOpen}
        disableEscapeKeyDown={false}
        onClose={() => setDialogOpen(false)}
      >
        <DialogHeader>
          <DialogTitle>Change password</DialogTitle>
        </DialogHeader>
        <DialogContent mb={0}>
          <FormPassword
            showCancel
            auth2faType={auth2faType}
            preferredMfaType={preferredMfaType}
            onChangePass={onChangePass}
            onChangePassWithWebauthn={onChangePassWithWebauthn}
            onCancel={() => setDialogOpen(false)}
          />
        </DialogContent>
      </Dialog>
    </Box>
  );
}
