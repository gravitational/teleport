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

import { useEffect, useState } from 'react';
import { matchPath } from 'react-router';

import { TrustedDeviceRequirement } from 'gen-proto-ts/teleport/legacy/types/trusted_device_requirement_pb';
import { useAttempt } from 'shared/hooks';
import { AuthProvider } from 'shared/services';

import cfg from 'teleport/config';
import auth, { UserCredentials } from 'teleport/services/auth';
import history from 'teleport/services/history';
import { storageService } from 'teleport/services/storageService';
import session from 'teleport/services/websession';

export default function useLogin() {
  const [attempt, attemptActions] = useAttempt({ isProcessing: false });
  const [checkingValidSession, setCheckingValidSession] = useState(true);
  const licenseAcknowledged = storageService.getLicenseAcknowledged();

  const authProviders = cfg.getAuthProviders();
  const auth2faType = cfg.getAuth2faType();
  const defaultConnectorName = cfg.getDefaultConnectorName();
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

  // onSuccess can receive a device webtoken. If so, it will
  // enable a prompt to allow users to authorize the current
  function onSuccess({
    deviceWebToken,
    trustedDeviceRequirement,
  }: LoginResponse) {
    // deviceWebToken will only exist on a login response
    // from enterprise but just in case there is a version mismatch
    // between the webclient and proxy
    if (trustedDeviceRequirement === TrustedDeviceRequirement.REQUIRED) {
      session.setDeviceTrustRequired();
    }
    if (deviceWebToken && cfg.isEnterprise) {
      return authorizeWithDeviceTrust(deviceWebToken);
    }
    return loginSuccess();
  }

  useEffect(() => {
    if (session.isValid()) {
      try {
        const redirectUrlWithBase = new URL(getEntryRoute());
        const matched = matchPath(redirectUrlWithBase.pathname, {
          path: cfg.routes.samlIdpSso,
          strict: true,
          exact: true,
        });
        if (matched) {
          history.push(redirectUrlWithBase, true);
          return;
        } else {
          history.replace(cfg.routes.root);
          return;
        }
      } catch (e) {
        console.error(e);
        history.replace(cfg.routes.root);
        return;
      }
      history.replace(cfg.routes.root);
      return;
    }
    setCheckingValidSession(false);
  }, []);

  function onLogin(email, password, token) {
    attemptActions.start();
    storageService.clearLoginTime();
    auth
      .login(email, password, token)
      .then(onSuccess)
      .catch(err => {
        attemptActions.error(err);
      });
  }

  function onLoginWithWebauthn(creds?: UserCredentials) {
    attemptActions.start();
    storageService.clearLoginTime();
    auth
      .loginWithWebauthn(creds)
      .then(onSuccess)
      .catch(err => {
        attemptActions.error(err);
      });
  }

  function onLoginWithSso(provider: AuthProvider) {
    attemptActions.start();
    storageService.clearLoginTime();
    const appStartRoute = getEntryRoute();
    const ssoUri = cfg.getSsoUrl(provider.url, provider.name, appStartRoute);
    history.push(ssoUri, true);
  }

  // Move the default connector to the front of the list so that it shows up at the top.
  const sortedProviders = moveToFront(
    authProviders,
    p => p.name === defaultConnectorName
  );

  return {
    attempt,
    onLogin,
    checkingValidSession,
    onLoginWithSso,
    authProviders: sortedProviders,
    auth2faType,
    preferredMfaType: cfg.getPreferredMfaType(),
    isLocalAuthEnabled,
    onLoginWithWebauthn,
    clearAttempt: attemptActions.clear,
    isPasswordlessEnabled: cfg.isPasswordlessEnabled(),
    primaryAuthType: cfg.getPrimaryAuthType(),
    licenseAcknowledged,
    setLicenseAcknowledged: storageService.setLicenseAcknowledged,
    motd,
    showMotd,
    acknowledgeMotd,
  };
}

type DeviceWebToken = {
  id: string;
  token: string;
};

type LoginResponse = {
  deviceWebToken?: DeviceWebToken;
  trustedDeviceRequirement?: TrustedDeviceRequirement;
};

function authorizeWithDeviceTrust(token: DeviceWebToken) {
  let redirect = history.getRedirectParam();
  const authorize = cfg.getDeviceTrustAuthorizeRoute(
    token.id,
    token.token,
    redirect
  );
  history.push(authorize, true);
}

function loginSuccess() {
  const redirect = getEntryRoute();
  const withPageRefresh = true;
  history.push(redirect, withPageRefresh);
}

/**
 * getEntryRoute returns a base ensured redirect URL value that is safe
 * for redirect.
 * @returns base ensured URL string.
 */
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

/**
 * moveToFront returns a copy of an array with the element that matches the condition to the front of it.
 */
function moveToFront<T>(arr: T[], condition: (item: T) => boolean): T[] {
  const copy = [...arr];
  const index = copy.findIndex(condition);

  if (index > 0) {
    const [item] = copy.splice(index, 1);
    copy.unshift(item);
  }

  return copy;
}
