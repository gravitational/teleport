/*
Copyright 2021-2022 Gravitational, Inc.

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
import PasswordForm from 'shared/components/FormPassword';

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
      <PasswordForm
        auth2faType={auth2faType}
        preferredMfaType={preferredMfaType}
        onChangePass={changePassword}
        onChangePassWithWebauthn={changePasswordWithWebauthn}
      />
    </FeatureBox>
  );
}
