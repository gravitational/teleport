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

import { useState, useEffect, useCallback } from 'react';

import { EventEmitterMfaSender } from 'teleport/lib/EventEmitterMfaSender';
import { TermEvent } from 'teleport/lib/term/enums';
import {
  makeMfaAuthenticateChallenge,
  makeWebauthnAssertionResponse,
} from 'teleport/services/mfa/makeMfa';
import { SsoChallenge } from 'teleport/services/mfa';
import auth from 'teleport/services/auth/auth';

export function useMfa(emitterSender: EventEmitterMfaSender): MfaState {
  const [state, setState] = useState<{
    errorText: string;
    addMfaToScpUrls: boolean;
    webauthnPublicKey: PublicKeyCredentialRequestOptions;
    ssoChallenge: SsoChallenge;
    totpChallenge: boolean;
  }>({
    addMfaToScpUrls: false,
    errorText: '',
    webauthnPublicKey: null,
    ssoChallenge: null,
    totpChallenge: false,
  });

  function clearChallenges() {
    setState(prevState => ({
      ...prevState,
      totpChallenge: false,
      webauthnPublicKey: null,
      ssoChallenge: null,
    }));
  }

  function onSsoAuthenticate() {
    if (!state.ssoChallenge) {
      setState(prevState => ({
        ...prevState,
        errorText: 'Invalid or missing SSO challenge',
      }));
      return;
    }

    auth.openSsoChallengeRedirect(state.ssoChallenge);
  }

  function onWebauthnAuthenticate() {
    if (!window.PublicKeyCredential) {
      const errorText =
        'This browser does not support WebAuthn required for hardware tokens, \
      please try the latest version of Chrome, Firefox or Safari.';

      setState({
        ...state,
        errorText,
      });
      return;
    }

    navigator.credentials
      .get({ publicKey: state.webauthnPublicKey })
      .then(res => {
        setState(prevState => ({
          ...prevState,
          errorText: '',
          webauthnPublicKey: null,
        }));
        const credential = makeWebauthnAssertionResponse(res);
        emitterSender.sendWebAuthn(credential);
      })
      .catch((err: Error) => {
        setErrorText(err.message);
      });
  }

  const waitForSsoChallengeResponse = useCallback(
    async (
      ssoChallenge: SsoChallenge,
      abortSignal: AbortSignal
    ): Promise<void> => {
      try {
        const resp = await auth.waitForSsoChallengeResponse(
          ssoChallenge,
          abortSignal
        );
        emitterSender.sendChallengeResponse(resp);
        clearChallenges();
      } catch (error) {
        if (error.name !== 'AbortError') {
          throw error;
        }
      }
    },
    [emitterSender]
  );

  useEffect(() => {
    let ssoChallengeAbortController: AbortController | undefined;
    const challengeHandler = (challengeJson: string) => {
      const { webauthnPublicKey, ssoChallenge, totpChallenge } =
        makeMfaAuthenticateChallenge(challengeJson);

      setState(prevState => ({
        ...prevState,
        addMfaToScpUrls: true,
        ssoChallenge,
        webauthnPublicKey,
        totpChallenge,
      }));

      if (ssoChallenge) {
        ssoChallengeAbortController = new AbortController();
        void waitForSsoChallengeResponse(
          ssoChallenge,
          ssoChallengeAbortController.signal
        );
      }
    };

    emitterSender?.on(TermEvent.MFA_CHALLENGE, challengeHandler);

    return () => {
      ssoChallengeAbortController?.abort();
      emitterSender?.removeListener(TermEvent.MFA_CHALLENGE, challengeHandler);
    };
  }, [emitterSender, waitForSsoChallengeResponse]);

  function setErrorText(newErrorText: string) {
    setState(prevState => ({ ...prevState, errorText: newErrorText }));
  }

  // if any challenge exists, requested is true
  const requested = !!(
    state.webauthnPublicKey ||
    state.totpChallenge ||
    state.ssoChallenge
  );

  return {
    requested,
    onWebauthnAuthenticate,
    onSsoAuthenticate,
    addMfaToScpUrls: state.addMfaToScpUrls,
    setErrorText,
    errorText: state.errorText,
    webauthnPublicKey: state.webauthnPublicKey,
    ssoChallenge: state.ssoChallenge,
  };
}

export type MfaState = {
  onWebauthnAuthenticate: () => void;
  onSsoAuthenticate: () => void;
  setErrorText: (errorText: string) => void;
  errorText: string;
  requested: boolean;
  addMfaToScpUrls: boolean;
  webauthnPublicKey: PublicKeyCredentialRequestOptions;
  ssoChallenge: SsoChallenge;
};

// used for testing
export function makeDefaultMfaState(): MfaState {
  return {
    onWebauthnAuthenticate: () => null,
    onSsoAuthenticate: () => null,
    setErrorText: () => null,
    errorText: '',
    requested: false,
    addMfaToScpUrls: false,
    webauthnPublicKey: null,
    ssoChallenge: null,
  };
}

function waitForMessage(
  channel: BroadcastChannel,
  abortSignal: AbortSignal
): Promise<MessageEvent> {
  return new Promise((resolve, reject) => {
    // Create the event listener
    function eventHandler(e: MessageEvent) {
      // Remove the event listener after it triggers
      channel.removeEventListener('message', eventHandler);
      // Resolve the promise with the event object
      resolve(e);
    }

    // Add the event listener
    channel.addEventListener('message', eventHandler);
    abortSignal.onabort = e => {
      channel.removeEventListener('message', eventHandler);
      reject(e);
    };
  });
}
