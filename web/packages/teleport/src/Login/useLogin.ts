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

import { useState, useEffect } from 'react';
import { useAttempt } from 'shared/hooks';
import { AuthProvider } from 'shared/services';

import session from 'teleport/services/websession';
import history from 'teleport/services/history';
import cfg from 'teleport/config';
import auth, { UserCredentials } from 'teleport/services/auth';

export default function useLogin() {
  const [attempt, attemptActions] = useAttempt({ isProcessing: false });
  const [checkingValidSession, setCheckingValidSession] = useState(true);

  const authProviders = cfg.getAuthProviders();
  const auth2faType = cfg.getAuth2faType();
  const isLocalAuthEnabled = cfg.getLocalAuthFlag();
  const motd = cfg.getMotd();
  const [showMotd, setShowMotd] = useState<boolean>(() => {
    const redirectUri = history.getRedirectParam();

    if (redirectUri?.includes('headless')) {
      return false;
    }
    return !!cfg.getMotd();
  });

  function acknowledgeMotd() {
    setShowMotd(false);
  }

  useEffect(() => {
    if (session.isValid()) {
      history.replace(cfg.routes.root);
      return;
    }
    setCheckingValidSession(false);
  }, []);

  function onLogin(email, password, token) {
    attemptActions.start();
    auth
      .login(email, password, token)
      .then(onSuccess)
      .catch(err => {
        attemptActions.error(err);
      });
  }

  function onLoginWithWebauthn(creds?: UserCredentials) {
    attemptActions.start();
    auth
      .loginWithWebauthn(creds)
      .then(onSuccess)
      .catch(err => {
        attemptActions.error(err);
      });
  }

  function onLoginWithSso(provider: AuthProvider) {
    attemptActions.start();
    const appStartRoute = getEntryRoute();
    const ssoUri = cfg.getSsoUrl(provider.url, provider.name, appStartRoute);
    history.push(ssoUri, true);
  }

  return {
    attempt,
    onLogin,
    checkingValidSession,
    onLoginWithSso,
    authProviders,
    auth2faType,
    preferredMfaType: cfg.getPreferredMfaType(),
    isLocalAuthEnabled,
    onLoginWithWebauthn,
    clearAttempt: attemptActions.clear,
    isPasswordlessEnabled: cfg.isPasswordlessEnabled(),
    primaryAuthType: cfg.getPrimaryAuthType(),
    motd,
    showMotd,
    acknowledgeMotd,
  };
}

function onSuccess() {
  const redirect = getEntryRoute();
  const withPageRefresh = true;
  history.push(redirect, withPageRefresh);
}

function getEntryRoute() {
  let entryUrl = history.getRedirectParam();
  if (entryUrl) {
    entryUrl = history.ensureKnownRoute(entryUrl);
  } else {
    entryUrl = cfg.routes.root;
  }

  return history.ensureBaseUrl(entryUrl);
}

export type State = ReturnType<typeof useLogin> & {
  isRecoveryEnabled?: boolean;
  onRecover?: (isRecoverPassword: boolean) => void;
};
