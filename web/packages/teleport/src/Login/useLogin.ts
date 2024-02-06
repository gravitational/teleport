/**
 * Copyright 2021-2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
