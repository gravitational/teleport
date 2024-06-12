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

import logoSrc from 'design/assets/images/teleport-medallion.svg';

import FormLogin from 'teleport/components/FormLogin';
import Logo from 'teleport/components/LogoHero';

import useLogin, { State } from './useLogin';
import Motd from './Motd';

export function Login() {
  const state = useLogin();
  return <LoginComponent {...state} />;
}

export function LoginComponent({
  attempt,
  onLogin,
  onLoginWithWebauthn,
  onLoginWithSso,
  authProviders,
  auth2faType,
  checkingValidSession,
  preferredMfaType,
  isLocalAuthEnabled,
  clearAttempt,
  isPasswordlessEnabled,
  primaryAuthType,
  motd,
  showMotd,
  acknowledgeMotd,
}: State) {
  // while we are checking if a session is valid, we don't return anything
  // to prevent flickering. The check only happens for a frame or two so
  // we avoid rendering a loader/indicator since that will flicker as well
  if (checkingValidSession) {
    return null;
  }
  return (
    <>
      <Logo src={logoSrc} />
      {showMotd ? (
        <Motd message={motd} onClick={acknowledgeMotd} />
      ) : (
        <FormLogin
          title={'Sign in to Teleport'}
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
        />
      )}
    </>
  );
}
