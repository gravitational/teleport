/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import {
  MfaAuthenticateChallenge,
  MfaAuthenticateChallengeJson,
  MfaRegistrationChallenge,
  MfaRegistrationChallengeJson,
  WebauthnAssertionResponse,
  WebauthnAttestationResponse,
} from './types';

// parseMfaRegistrationChallengeJson formats fetched register challenge JSON.
export function parseMfaRegistrationChallengeJson(
  challenge: MfaRegistrationChallengeJson
): MfaRegistrationChallenge {
  // WebAuthn challenge contains Base64URL(byte) fields that needs to
  // be converted to ArrayBuffer expected by navigator.credentials.create:
  // - challenge
  // - user.id
  // - excludeCredentials[i].id
  const webauthnPublicKeyFromJson = (
    json: PublicKeyCredentialCreationOptionsJSON
  ) =>
    ({
      ...json,
      challenge: base64urlToBuffer(json.challenge),
      user: {
        ...json.user,
        id: base64urlToBuffer(json.user?.id || ''),
      },
      excludeCredentials: json.excludeCredentials?.map(credential => ({
        ...credential,
        id: base64urlToBuffer(credential.id),
      })),
    }) as PublicKeyCredentialCreationOptions;

  return {
    qrCode: challenge.totp?.qrCode,
    webauthnPublicKey: challenge.webauthn
      ? webauthnPublicKeyFromJson(challenge.webauthn.publicKey)
      : null,
  };
}

// parseMfaChallengeJson formats fetched authenticate challenge JSON.
export function parseMfaChallengeJson(
  challenge: MfaAuthenticateChallengeJson
): MfaAuthenticateChallenge | undefined {
  if (
    !challenge.sso_challenge &&
    !challenge.webauthn_challenge &&
    !challenge.totp_challenge
  ) {
    return;
  }

  // WebAuthn challenge contains Base64URL(byte) fields that needs to
  // be converted to ArrayBuffer expected by navigator.credentials.get:
  // - challenge
  // - allowCredentials[i].id
  const webauthnPublicKeyFromJson = (
    json: PublicKeyCredentialRequestOptionsJSON
  ) =>
    ({
      ...json,
      challenge: base64urlToBuffer(json.challenge),
      allowCredentials: json.allowCredentials?.map(credential => ({
        ...credential,
        id: base64urlToBuffer(credential.id),
      })),
    }) as PublicKeyCredentialRequestOptions;

  return {
    ssoChallenge: challenge.sso_challenge,
    totpChallenge: challenge.totp_challenge,
    webauthnPublicKey: challenge.webauthn_challenge
      ? webauthnPublicKeyFromJson(challenge.webauthn_challenge.publicKey)
      : null,
  };
}

// makeWebauthnCreationResponse takes a credential returned from navigator.credentials.create
// and returns the credential attestation response.
export function makeWebauthnCreationResponse(
  cred: Credential
): WebauthnAttestationResponse {
  const publicKey = cred as PublicKeyCredential;

  // Response can be null if no Credential object can be created.
  if (!publicKey) {
    throw new Error('error creating credential, please try again');
  }

  const clientExtentions = publicKey.getClientExtensionResults();
  const attestationResponse =
    publicKey.response as AuthenticatorAttestationResponse;

  return {
    id: cred.id,
    type: cred.type,
    extensions: {
      appid: Boolean(clientExtentions?.appid),
      credProps: clientExtentions?.credProps,
    },
    rawId: bufferToBase64url(publicKey.rawId),
    response: {
      attestationObject: bufferToBase64url(
        attestationResponse?.attestationObject
      ),
      clientDataJSON: bufferToBase64url(attestationResponse?.clientDataJSON),
    },
  };
}

// makeWebauthnAssertionResponse takes a credential returned from navigator.credentials.get
// and returns the credential assertion response.
export function makeWebauthnAssertionResponse(
  cred: Credential
): WebauthnAssertionResponse {
  const publicKey = cred as PublicKeyCredential;

  // Response can be null if Credential cannot be unambiguously obtained.
  if (!publicKey) {
    throw new Error(
      'error obtaining credential from the hardware key, please try again'
    );
  }

  const clientExtentions = publicKey.getClientExtensionResults();
  const assertionResponse =
    publicKey.response as AuthenticatorAssertionResponse;

  return {
    id: cred.id,
    type: cred.type,
    extensions: {
      appid: Boolean(clientExtentions?.appid),
    },
    rawId: bufferToBase64url(publicKey.rawId),
    response: {
      authenticatorData: bufferToBase64url(
        assertionResponse?.authenticatorData
      ),
      clientDataJSON: bufferToBase64url(assertionResponse?.clientDataJSON),
      signature: bufferToBase64url(assertionResponse?.signature),
      userHandle: bufferToBase64url(assertionResponse?.userHandle),
    },
  };
}
