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

import { useState, useEffect, Dispatch, SetStateAction } from 'react';

import { EventEmitterWebAuthnSender } from 'teleport/lib/EventEmitterWebAuthnSender';
import { TermEvent } from 'teleport/lib/term/enums';
import {
  makeMfaAuthenticateChallenge,
  makeWebauthnAssertionResponse,
} from 'teleport/services/auth';

export default function useWebAuthn(
  emitterSender: EventEmitterWebAuthnSender
): WebAuthnState {
  const [state, setState] = useState({
    addMfaToScpUrls: false,
    requested: false,
    errorText: '',
    publicKey: null as PublicKeyCredentialRequestOptions,
  });

  function authenticate() {
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

  const onChallenge = challengeJson => {
    const challenge = JSON.parse(challengeJson);
    const publicKey = makeMfaAuthenticateChallenge(challenge).webauthnPublicKey;

    setState({
      ...state,
      requested: true,
      addMfaToScpUrls: true,
      publicKey,
    });
  };

  useEffect(() => {
    if (emitterSender) {
      emitterSender.on(TermEvent.WEBAUTHN_CHALLENGE, onChallenge);

      return () => {
        emitterSender.removeListener(TermEvent.WEBAUTHN_CHALLENGE, onChallenge);
      };
    }
  }, [emitterSender]);

  return {
    errorText: state.errorText,
    requested: state.requested,
    authenticate,
    setState,
    addMfaToScpUrls: state.addMfaToScpUrls,
  };
}

export type WebAuthnState = {
  errorText: string;
  requested: boolean;
  authenticate: () => void;
  setState: Dispatch<
    SetStateAction<{
      addMfaToScpUrls: boolean;
      requested: boolean;
      errorText: string;
      publicKey: PublicKeyCredentialRequestOptions;
    }>
  >;
  addMfaToScpUrls: boolean;
};
