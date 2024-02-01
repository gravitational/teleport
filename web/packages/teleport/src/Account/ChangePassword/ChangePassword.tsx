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

import React from 'react';
import PasswordForm from 'shared/components/FormPassword';

import { Card } from 'design';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';

import useChangePassword, { State } from './useChangePassword';

export default function Container() {
  const state = useChangePassword();
  return <ChangePassword {...state} />;
}

export function ChangePassword({
  changePassword,
  changePasswordWithWebauthn,
  preferredMfaType,
  auth2faType,
}: State) {
  return (
    <FeatureBox style={{ padding: 0 }}>
      <FeatureHeader border="none">
        <FeatureHeaderTitle>Change Password</FeatureHeaderTitle>
      </FeatureHeader>
      <Card width="456px" p="6">
        <PasswordForm
          auth2faType={auth2faType}
          preferredMfaType={preferredMfaType}
          onChangePass={changePassword}
          onChangePassWithWebauthn={changePasswordWithWebauthn}
        />
      </Card>
    </FeatureBox>
  );
}
