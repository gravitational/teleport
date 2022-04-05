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

import { useAttempt } from 'shared/hooks';
import history from 'teleport/services/history';
import cfg from 'teleport/config';
import auth from 'teleport/services/auth';
import { AuthProvider } from 'shared/services';

export default function useLogin() {
  const [attempt, attemptActions] = useAttempt({ isProcessing: false });
  const authProviders = cfg.getAuthProviders();
  const auth2faType = cfg.getAuth2faType();
  const isLocalAuthEnabled = cfg.getLocalAuthFlag();

  function onLogin(email, password, token) {
    attemptActions.start();
    auth
      .login(email, password, token)
      .then(onSuccess)
      .catch(err => {
        attemptActions.error(err);
      });
  }

  function onLoginWithWebauthn(name, password) {
    attemptActions.start();
    auth
      .loginWithWebauthn(name, password)
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
    onLoginWithSso,
    authProviders,
    auth2faType,
    preferredMfaType: cfg.getPreferredMfaType(),
    isLocalAuthEnabled,
    onLoginWithWebauthn,
    clearAttempt: attemptActions.clear,
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
