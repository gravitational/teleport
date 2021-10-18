/**
 * Copyright 2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { Base64urlString } from 'shared/utils/base64url';

export type DeviceType = 'totp' | 'u2f' | 'webauthn';

export type UserCredentials = {
  username: string;
  password: string;
};

export type AuthnChallengeRequest = {
  tokenId?: string;
  userCred: UserCredentials;
};

export type U2fRegisterRequest = {
  version: string;
  challenge: string;
  appId: string;
};

export type U2fSignRequest = {
  version: string;
  challenge: string;
  keyhandle: string;
  appId: string;
};

export type MfaAuthenticateChallenge = {
  u2fSignRequests: U2fSignRequest[];
  webauthnPublicKey: PublicKeyCredentialRequestOptions;
};

export type MfaRegistrationChallenge = {
  qrCode: Base64urlString;
  u2fRegisterRequest: U2fRegisterRequest;
  webauthnPublicKey: PublicKeyCredentialCreationOptions;
};
