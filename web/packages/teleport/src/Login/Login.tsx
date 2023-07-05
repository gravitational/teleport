/*
Copyright 2019-2022 Gravitational, Inc.

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

import FormLogin from 'teleport/components/FormLogin';
import Logo from 'teleport/components/LogoHero';

import useLogin, { State } from './useLogin';
import Motd from './Motd';

const logoSrc = require('design/assets/images/teleport-medallion.svg');

export default function Container() {
  const state = useLogin();
  return <Login {...state} />;
}

export function Login({
  attempt,
  onLogin,
  onLoginWithWebauthn,
  onLoginWithSso,
  authProviders,
  auth2faType,
  preferredMfaType,
  isLocalAuthEnabled,
  clearAttempt,
  isPasswordlessEnabled,
  primaryAuthType,
  privateKeyPolicyEnabled,
  motd,
  showMotd,
  acknowledgeMotd,
}: State) {
  return (
    <>
      <Logo src={logoSrc} />
      {showMotd ? (
        <Motd message={motd} onClick={acknowledgeMotd} />
      ) : (
        <FormLogin
          title={'Sign into Teleport'}
          authProviders={authProviders}
          auth2faType={auth2faType}
          preferredMfaType={preferredMfaType}
          isLocalAuthEnabled={isLocalAuthEnabled}
          onLoginWithSso={onLoginWithSso}
          onLoginWithWebauthn={onLoginWithWebauthn}
          onLogin={onLogin}
          attempt={attempt}
          clearAttempt={clearAttempt}
          isPasswordlessEnabled={isPasswordlessEnabled}
          primaryAuthType={primaryAuthType}
          privateKeyPolicyEnabled={privateKeyPolicyEnabled}
        />
      )}
    </>
  );
}
