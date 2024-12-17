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
import { parseMfaChallengeJson as parseMfaChallenge } from 'teleport/services/mfa/makeMfa';
import {
  MfaAuthenticateChallengeJson,
  SSOChallenge,
} from 'teleport/services/mfa';
import auth from 'teleport/services/auth/auth';

export function useMfa(emitterSender: EventEmitterMfaSender): MfaState {
  const [state, setState] = useState<{
    errorText: string;
    addMfaToScpUrls: boolean;
    webauthnPublicKey: PublicKeyCredentialRequestOptions;
    ssoChallenge: SSOChallenge;
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

    // try to center the screen
    const width = 1045;
    const height = 550;
    const left = (screen.width - width) / 2;
    const top = (screen.height - height) / 2;

    // these params will open a tiny window.
    const params = `width=${width},height=${height},left=${left},top=${top}`;
    window.open(state.ssoChallenge.redirectUrl, '_blank', params);
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

    auth
      .getMfaChallengeResponse({
        webauthnPublicKey: state.webauthnPublicKey,
      })
      .then(res => {
        setState(prevState => ({
          ...prevState,
          errorText: '',
          webauthnPublicKey: null,
        }));
        emitterSender.sendWebAuthn(res.webauthn_response);
      })
      .catch((err: Error) => {
        setErrorText(err.message);
      });
  }

  const waitForSsoChallengeResponse = useCallback(
    async (
      ssoChallenge: SSOChallenge,
      abortSignal: AbortSignal
    ): Promise<void> => {
      const channel = new BroadcastChannel(ssoChallenge.channelId);

      try {
        const event = await waitForMessage(channel, abortSignal);
        emitterSender.sendChallengeResponse({
          sso_response: {
            requestId: ssoChallenge.requestId,
            token: event.data.mfaToken,
          },
        });
        clearChallenges();
      } catch (error) {
        if (error.name !== 'AbortError') {
          throw error;
        }
      } finally {
        channel.close();
      }
    },
    [emitterSender]
  );

  useEffect(() => {
    let ssoChallengeAbortController: AbortController | undefined;
    const challengeHandler = (challengeJson: string) => {
      const challenge = JSON.parse(
        challengeJson
      ) as MfaAuthenticateChallengeJson;

      const { webauthnPublicKey, ssoChallenge, totpChallenge } =
        parseMfaChallenge(challenge);

      setState(prevState => ({
        ...prevState,
        addMfaToScpUrls: true,
        ssoChallenge,
        webauthnPublicKey,
        totpChallenge,
      }));

      if (ssoChallenge) {
        ssoChallengeAbortController?.abort();
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
  ssoChallenge: SSOChallenge;
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
