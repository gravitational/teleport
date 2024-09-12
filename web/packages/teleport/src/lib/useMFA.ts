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

import { useCallback } from 'react';
import { useState, useEffect, Dispatch, SetStateAction } from 'react';

import { EventEmitterWebAuthnSender } from 'teleport/lib/EventEmitterWebAuthnSender';
import { TermEvent } from 'teleport/lib/term/enums';
import {
  makeMfaAuthenticateChallenge,
  makeWebauthnAssertionResponse,
} from 'teleport/services/auth';

type SSOasMFACallbacks = Record<string, VoidFunction>;

export default function useMFA(
  emitterSender: EventEmitterWebAuthnSender
): WebAuthnState {
  const [state, setState] = useState({
    addMfaToScpUrls: false,
    requested: false,
    errorText: '',
    publicKey: null as PublicKeyCredentialRequestOptions,
    redirectUri: '',
  });
  const [awaitingIdpResponse, setAwaitingIdpResponse] = useState(false);
  const [bc, setBc] = useState<BroadcastChannel>(
    new BroadcastChannel('sso_confirm')
  );
  const [mfaCallbacks, setMfaCallbacks] = useState<SSOasMFACallbacks>({});

  const registerMfaCallback = (key: string, cb: VoidFunction) => {
    const newCallbacks = { ...mfaCallbacks };
    newCallbacks[key] = cb;
    setMfaCallbacks(newCallbacks);
  };

  const unregisterMfaCallback = (key: string) => {
    const newCallbacks = mfaCallbacks;
    delete newCallbacks[key];
    setMfaCallbacks(newCallbacks);
  };

  useEffect(() => {
    function handleMessage() {
      bc.postMessage({ received: true });
      mfaCallbacks['session_mfa']?.();
    }

    bc.addEventListener('message', handleMessage);

    return () => {
      bc.removeEventListener('message', handleMessage);
      // TODO (avatus) : close this properly
      // bc.close();
    };
  }, [bc, mfaCallbacks]);

  function authenticateWebauthn() {
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
      .get({ publicKey: state.publicKey })
      .then(res => {
        const credential = makeWebauthnAssertionResponse(res);
        emitterSender.sendWebAuthn(credential);

        setState({
          ...state,
          requested: false,
          errorText: '',
        });
      })
      .catch((err: Error) => {
        setState({
          ...state,
          errorText: err.message,
        });
      });
  }

  function authenticateProvider() {
    setAwaitingIdpResponse(true);
    // window.location.href = state.redirectUri;
    // try to center the screen
    const width = 1045;
    const height = 550;
    const left = (screen.width - width) / 2;
    const top = (screen.height - height) / 2;

    // these params will open a tiny window. adjust as needed
    const params = `width=${width},height=${height},left=${left},top=${top}`;
    window.open(state.redirectUri, '_blank', params);
  }

  const onWebAuthnChallenge = challengeJson => {
    const challenge = JSON.parse(challengeJson);
    const publicKey = makeMfaAuthenticateChallenge(challenge).webauthnPublicKey;

    setState({
      ...state,
      requested: true,
      addMfaToScpUrls: true,
      publicKey,
    });
  };

  const onIdPChallenge = challengeJson => {
    const challenge = JSON.parse(challengeJson);

    setState({
      ...state,
      requested: true,
      addMfaToScpUrls: true, // i dont think we'll use this with IdP?
      redirectUri: challenge.idp_challenge.redirect_url,
    });
  };

  useEffect(() => {
    if (emitterSender) {
      emitterSender.on(TermEvent.WEBAUTHN_CHALLENGE, onWebAuthnChallenge);
      emitterSender.on(TermEvent.IDP_CHALLENGE, onIdPChallenge);

      return () => {
        emitterSender.removeListener(
          TermEvent.WEBAUTHN_CHALLENGE,
          onWebAuthnChallenge
        );
        emitterSender.removeListener(TermEvent.IDP_CHALLENGE, onIdPChallenge);
      };
    }
  }, [emitterSender]);

  return {
    errorText: state.errorText,
    requested: state.requested,
    authenticateWebauthn: authenticateWebauthn,
    authenticateProvider: authenticateProvider,
    registerMfaCallback,
    unregisterMfaCallback,
    awaitingIdpResponse,
    setState,
    addMfaToScpUrls: state.addMfaToScpUrls,
    publicKey: state.publicKey,
    redirectUri: state.redirectUri,
  };
}

export type WebAuthnState = {
  errorText: string;
  requested: boolean;
  registerMfaCallback: (key: string, cb: VoidFunction) => void;
  unregisterMfaCallback: (key: string) => void;
  authenticateWebauthn: () => void;
  authenticateProvider: () => void;
  setState: Dispatch<
    SetStateAction<{
      addMfaToScpUrls: boolean;
      requested: boolean;
      errorText: string;
      publicKey: PublicKeyCredentialRequestOptions;
    }>
  >;
  addMfaToScpUrls: boolean;
  awaitingIdpResponse: boolean;
  publicKey: PublicKeyCredentialRequestOptions;
  redirectUri: string;
};
