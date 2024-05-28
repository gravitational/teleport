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

import { base64urlToBuffer, bufferToBase64url } from 'shared/utils/base64';

import { MfaAuthenticateChallenge, MfaRegistrationChallenge } from './types';

// makeMfaRegistrationChallenge formats fetched register challenge JSON.
// Webauthn challange contains Base64URL(byte) fields that needs to
// be converted to ArrayBuffer expected by navigator.credentials.create:
// - challenge
// - user.id
// - excludeCredentials[i].id
export function makeMfaRegistrationChallenge(json): MfaRegistrationChallenge {
  const webauthnPublicKey = json.webauthn?.publicKey;
  if (webauthnPublicKey) {
    const challenge = webauthnPublicKey.challenge || '';
    const id = webauthnPublicKey.user?.id || '';
    const excludeCredentials = webauthnPublicKey.excludeCredentials || [];

    webauthnPublicKey.challenge = base64urlToBuffer(challenge);
    webauthnPublicKey.user.id = base64urlToBuffer(id);
    webauthnPublicKey.excludeCredentials = excludeCredentials.map(
      (credential, i) => {
        excludeCredentials[i].id = base64urlToBuffer(credential.id);
        return excludeCredentials[i];
      }
    );
  }

  return {
    qrCode: json.totp?.qrCode,
    webauthnPublicKey,
  };
}

// makeMfaAuthenticateChallenge formats fetched authenticate challenge JSON.
// Webauthn challange contains Base64URL(byte) fields that needs to
// be converted to ArrayBuffer expected by navigator.credentials.get:
// - challenge
// - allowCredentials[i].id
export function makeMfaAuthenticateChallenge(json): MfaAuthenticateChallenge {
  const webauthnPublicKey = json.webauthn_challenge?.publicKey;
  if (webauthnPublicKey) {
    const challenge = webauthnPublicKey.challenge || '';
    const allowCredentials = webauthnPublicKey.allowCredentials || [];

    webauthnPublicKey.challenge = base64urlToBuffer(challenge);
    webauthnPublicKey.allowCredentials = allowCredentials.map(
      (credential, i) => {
        allowCredentials[i].id = base64urlToBuffer(credential.id);
        return allowCredentials[i];
      }
    );
  }

  return {
    webauthnPublicKey: webauthnPublicKey,
  };
}

// makeWebauthnCreationResponse takes response from navigator.credentials.create
// and creates a credential object expected by the server with ArrayBuffer
// fields converted to Base64URL:
// - rawId
// - response.attestationObject
// - response.clientDataJSON
export function makeWebauthnCreationResponse(res) {
  // Response can be null if no Credential object can be created.
  if (!res) {
    throw new Error('error creating credential, please try again');
  }

  const clientExtentions = res.getClientExtensionResults();
  return {
    id: res.id,
    type: res.type,
    extensions: {
      appid: Boolean(clientExtentions?.appid),
    },
    rawId: bufferToBase64url(res.rawId),
    response: {
      attestationObject: bufferToBase64url(res.response?.attestationObject),
      clientDataJSON: bufferToBase64url(res.response?.clientDataJSON),
    },
  };
}

// makeWebauthnAssertionResponse takes response from navigator.credentials.get
// and creates a credential object expected by the server with ArrayBuffer
// fields converted to Base64URL:
// - rawId
// - response.authenticatorData
// - response.clientDataJSON
// - response.signature
// - response.userHandle
export function makeWebauthnAssertionResponse(res): WebauthnAssertionResponse {
  // Response can be null if Credential cannot be unambiguously obtained.
  if (!res) {
    throw new Error(
      'error obtaining credential from the hardware key, please try again'
    );
  }

  const clientExtentions = res.getClientExtensionResults();

  return {
    id: res.id,
    type: res.type,
    extensions: {
      appid: Boolean(clientExtentions?.appid),
    },
    rawId: bufferToBase64url(res.rawId),
    response: {
      authenticatorData: bufferToBase64url(res.response?.authenticatorData),
      clientDataJSON: bufferToBase64url(res.response?.clientDataJSON),
      signature: bufferToBase64url(res.response?.signature),
      userHandle: bufferToBase64url(res.response?.userHandle),
    },
  };
}

export type WebauthnAssertionResponse = {
  id: string;
  type: string;
  extensions: {
    appid: boolean;
  };
  rawId: string;
  response: {
    authenticatorData: string;
    clientDataJSON: string;
    signature: string;
    userHandle: string;
  };
};
