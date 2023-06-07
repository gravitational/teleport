/**
 * Copyright 2021-2022 Gravitational, Inc.
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
import { EventMeta } from 'teleport/services/userEvent';

export type Base64urlString = string;

export type UserCredentials = {
  username: string;
  password: string;
};

export type AuthnChallengeRequest = {
  tokenId?: string;
  userCred: UserCredentials;
};

export type MfaAuthenticateChallenge = {
  webauthnPublicKey: PublicKeyCredentialRequestOptions;
};

export type MfaRegistrationChallenge = {
  qrCode: Base64urlString;
  webauthnPublicKey: PublicKeyCredentialCreationOptions;
};

export type RecoveryCodes = {
  codes?: string[];
  createdDate: Date;
};

export type ChangedUserAuthn = {
  recovery: RecoveryCodes;
  privateKeyPolicyEnabled?: boolean;
};

export type NewCredentialRequest = {
  tokenId: string;
  password?: string;
  otpCode?: string;
  deviceName?: string;
};

export type ResetToken = {
  tokenId: string;
  qrCode: string;
  user: string;
};

export type ResetPasswordReqWithEvent = {
  req: NewCredentialRequest;
  eventMeta?: EventMeta;
};

export type ResetPasswordWithWebauthnReqWithEvent = {
  req: NewCredentialRequest;
  eventMeta?: EventMeta;
};
