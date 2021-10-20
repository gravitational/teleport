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

import React from 'react';
import cfg from 'teleport/config';
import PasswordForm from 'shared/components/FormPassword';
import { FeatureBox } from 'teleport/components/Layout';
import authService from 'teleport/services/auth';

export default function Container() {
  const state = useAccount();
  return <Account {...state} />;
}

export function Account(props: ReturnType<typeof useAccount>) {
  const {
    auth2faType,
    preferredMfaType,
    changePass,
    changePassWithU2f,
    changePassWithWebauthn,
  } = props;
  return (
    <FeatureBox pt="4">
      <PasswordForm
        auth2faType={auth2faType}
        preferredMfaType={preferredMfaType}
        onChangePass={changePass}
        onChangePassWithU2f={changePassWithU2f}
        onChangePassWithWebauthn={changePassWithWebauthn}
      />
    </FeatureBox>
  );
}

function useAccount() {
  return {
    auth2faType: cfg.getAuth2faType(),
    preferredMfaType: cfg.getPreferredMfaType(),
    changePass: authService.changePassword.bind(authService),
    changePassWithU2f: authService.changePasswordWithU2f.bind(authService),
    changePassWithWebauthn: authService.changePasswordWithWebauthn.bind(
      authService
    ),
  };
}
