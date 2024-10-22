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
  SSOChallenge,
} from 'teleport/services/auth';

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

  // TODO (avatus), this is stubbed for types but wont not be called
  // until SSO as MFA backend is in.
  function onSsoAuthenticate() {
    // eslint-disable-next-line no-console
    console.error('not yet implemented');
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

  const onChallenge = useCallback(challengeJson => {
    const { webauthnPublicKey, ssoChallenge, totpChallenge } =
      makeMfaAuthenticateChallenge(challengeJson);

    setState(prevState => ({
      ...prevState,
      ssoChallenge,
      webauthnPublicKey,
      totpChallenge,
    }));
  }, []);

  useEffect(() => {
    if (emitterSender) {
      emitterSender.on(TermEvent.WEBAUTHN_CHALLENGE, onChallenge);

      return () => {
        emitterSender.removeListener(TermEvent.WEBAUTHN_CHALLENGE, onChallenge);
      };
    }
  }, [emitterSender, onChallenge]);

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
