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

import { useEffect, useRef, useState } from 'react';
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

  const [autoPromptDisabled, setAutoPromptDisabledState] = useState(
    storageService.getPasskeyAutoPromptDisabled()
  );
  // Whether the auto-prompt is something this cluster can do at all, which is what decides whether the
  // opt-out is worth offering. It stays visible for browsers that have not been prompted yet, so the
  // opt-out does not require sitting through a prompt first.
  const autoPromptAvailable = cfg.isPasswordlessEnabled();
  // Prompting a browser that holds no passkey for this cluster opens a dialog the user cannot satisfy,
  // so the ceremony itself waits for a passwordless sign-in to have happened here.
  const autoPromptEligible =
    autoPromptAvailable && storageService.getHasLoggedInWithPasskey();
  // Guards the auto-prompt so the passkey ceremony is invoked at most once per
  // page load, even as checkingValidSession flips and the effect re-runs.
  const autoPromptFired = useRef(false);
  // A browser rejects a second credential ceremony while one is outstanding, so the automatic one has
  // to stand down as soon as the user commits to signing in some other way.
  const autoPromptAbort = useRef<AbortController>(null);
  const [autoPromptPending, setAutoPromptPending] = useState(false);

  function abortAutoPrompt() {
    autoPromptAbort.current?.abort();
    autoPromptAbort.current = null;
    setAutoPromptPending(false);
  }

  // A ceremony left open after the page goes away would keep prompting for a sign-in nobody is
  // waiting on.
  useEffect(() => abortAutoPrompt, []);

  function setAutoPromptDisabled(disabled: boolean) {
    storageService.setPasskeyAutoPromptDisabled(disabled);
    setAutoPromptDisabledState(disabled);
  }

  const authProviders = cfg.getAuthProviders();
  const auth2faType = cfg.getAuth2faType();
  const defaultConnectorName = cfg.getDefaultConnectorName();
  const isLocalAuthEnabled = cfg.getLocalAuthFlag();
  const motd = cfg.getMotd();
  const [showMotd, setShowMotd] = useState<boolean>(() => {
    const redirectUri = history.getRedirectParam();

    if (
      redirectUri?.includes('headless') ||
      redirectUri?.includes('/mfa/browser')
    ) {
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
        const matched = matchPath(
          cfg.routes.samlIdpSso,
          redirectUrlWithBase.pathname
        );
        if (matched) {
          history.push(redirectUrlWithBase.toString(), true);
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
    }
    setCheckingValidSession(false);
  }, []);

  // Auto-invoke the passwordless ceremony once the session check confirms there
  // is no valid session, but only for browsers that have signed in with a
  // passkey before and have not opted out. Some browsers gate the modal
  // credentials.get() on user activation, in which case the ceremony is
  // rejected and the auto-prompt leaves the form as it was.
  useEffect(() => {
    if (checkingValidSession || autoPromptFired.current) {
      return;
    }
    // The MOTD and the community license both have to be accepted before signing
    // in, and Login renders them in place of the form, so a ceremony here would
    // authenticate the user past a gate they never saw. Mirrors the conditions
    // Login uses to pick what it renders.
    if (showMotd || (!licenseAcknowledged && cfg.edition === 'community')) {
      return;
    }
    if (autoPromptEligible && !autoPromptDisabled) {
      autoPromptFired.current = true;
      autoPromptAbort.current = new AbortController();
      setAutoPromptPending(true);
      loginWithWebauthn(
        undefined /* creds */,
        true /* auto */,
        autoPromptAbort.current.signal
      );
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [checkingValidSession, showMotd, licenseAcknowledged]);

  function onLogin(email, password, token) {
    abortAutoPrompt();
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
    abortAutoPrompt();
    loginWithWebauthn(creds, false /* auto */);
  }

  function loginWithWebauthn(
    creds: UserCredentials | undefined,
    auto: boolean,
    signal?: AbortSignal
  ) {
    const isPasswordless = !creds;
    // The auto-prompt stays out of the form's attempt state entirely. Claiming it would disable every
    // field behind a dialog the user never asked for, and releasing it again on failure would discard
    // the state of a sign-in they started in the meantime.
    if (!auto) {
      attemptActions.start();
      storageService.clearLoginTime();
    }
    auth
      .loginWithWebauthn(creds, signal)
      .then(res => {
        // Authenticated only records a login time when none is stored, so the previous session's has
        // to go before this one starts.
        if (auto) {
          storageService.clearLoginTime();
        }
        if (isPasswordless) {
          storageService.setHasLoggedInWithPasskey();
        }
        return onSuccess(res);
      })
      .catch(err => {
        // A dismissed dialog, an abort, or a browser that refuses a ceremony without user activation
        // are all expected outcomes of a prompt nobody requested. Leave the form as the user found it.
        if (auto) {
          return;
        }
        attemptActions.error(err);
      })
      .finally(() => {
        if (auto) {
          setAutoPromptPending(false);
        }
      });
  }

  function onLoginWithSso(provider: AuthProvider, loginHint?: string) {
    abortAutoPrompt();
    attemptActions.start();
    storageService.clearLoginTime();
    const appStartRoute = getEntryRoute();
    const ssoUri = cfg.getSsoUrl(
      provider.url,
      provider.name,
      appStartRoute,
      loginHint
    );
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
    autoPromptAvailable,
    autoPromptPending,
    autoPromptDisabled,
    setAutoPromptDisabled,
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
