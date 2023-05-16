/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { useState, useEffect } from 'react';

import { EventEmitterWebAuthnSender } from 'teleport/lib/EventEmitterWebAuthnSender';
import { TermEvent } from 'teleport/lib/term/enums';
import {
  makeMfaAuthenticateChallenge,
  makeWebauthnAssertionResponse,
} from 'teleport/services/auth';

export default function useWebAuthn(emitterSender: EventEmitterWebAuthnSender) {
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
