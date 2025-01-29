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

import { AuthProviderType } from 'shared/services';

import { Base64urlString, CreateNewHardwareDeviceRequest } from '../auth/types';

export type DeviceType = 'totp' | 'webauthn' | 'sso';

/** The intended usage of the device (as an MFA method or a passkey). */
export type DeviceUsage = 'passwordless' | 'mfa';

export interface MfaDevice {
  id: string;
  name: string;
  description: string;
  registeredDate: Date;
  lastUsedDate: Date;
  type: DeviceType;
  usage: DeviceUsage;
}

export type AddNewTotpDeviceRequest = {
  tokenId: string;
  deviceName: string;
  secondFactorToken: string;
};

export type AddNewHardwareDeviceRequest = CreateNewHardwareDeviceRequest & {
  deviceName: string;
};

export type SaveNewHardwareDeviceRequest = {
  addRequest: AddNewHardwareDeviceRequest;
  credential: Credential;
};

export type MfaAuthenticateChallengeJson = {
  sso_challenge?: SsoChallenge;
  totp_challenge?: boolean;
  webauthn_challenge?: {
    publicKey: PublicKeyCredentialRequestOptionsJSON;
  };
};

export type MfaAuthenticateChallenge = {
  ssoChallenge?: SsoChallenge;
  totpChallenge?: boolean;
  webauthnPublicKey?: PublicKeyCredentialRequestOptions;
};

export type SsoChallenge = {
  channelId: string;
  redirectUrl: string;
  requestId: string;
  device: {
    connectorId: string;
    connectorType: AuthProviderType;
    displayName: string;
  };
};

export type MfaRegistrationChallengeJson = {
  totp?: {
    qrCode: Base64urlString;
  };
  webauthn?: {
    publicKey: PublicKeyCredentialCreationOptionsJSON;
  };
};

export type MfaRegistrationChallenge = {
  qrCode: Base64urlString;
  webauthnPublicKey: PublicKeyCredentialCreationOptions;
};

export type MfaChallengeResponse = {
  totp_code?: string;
  webauthn_response?: WebauthnAssertionResponse;
  sso_response?: SsoChallengeResponse;
};

export type SsoChallengeResponse = {
  requestId: string;
  token: string;
};

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

export type WebauthnAttestationResponse = {
  id: string;
  type: string;
  extensions: {
    appid: boolean;
    credProps: CredentialPropertiesOutput;
  };
  rawId: string;
  response: {
    attestationObject: string;
    clientDataJSON: string;
  };
};
