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

import type {
  DeviceUsage,
  IsMfaRequiredRequest,
  MfaChallengeResponse,
  MfaChallengeScope,
} from '../mfa/types';
import type { EventMeta } from '../userEvent/types';

export type Base64urlString = string;

export interface UserCredentials {
  username: string;
  password: string;
}

export interface RecoveryCodes {
  codes?: string[];
  createdDate: Date;
}

export interface ChangedUserAuthn {
  recovery: RecoveryCodes;
}

/** A Web API request data for the New Credentials call. */
export interface NewCredentialRequest {
  tokenId: string;
  password?: string;
  otpCode?: string;
  deviceName?: string;
}

export interface ResetToken {
  tokenId: string;
  qrCode: string;
  user: string;
}

export interface ResetPasswordReqWithEvent {
  req: NewCredentialRequest;
  eventMeta?: EventMeta;
}

export interface ResetPasswordWithWebauthnReqWithEvent {
  req: NewCredentialRequest;
  credential?: Credential;
  eventMeta?: EventMeta;
}

export interface CreateAuthenticateChallengeRequest {
  scope: MfaChallengeScope;
  allowReuse?: boolean;
  isMfaRequiredRequest?: IsMfaRequiredRequest;
  userVerificationRequirement?: UserVerificationRequirement;
}

export interface ChangePasswordReq {
  oldPassword: string;
  newPassword: string;
  mfaResponse?: MfaChallengeResponse;
}

export interface CreateNewHardwareDeviceRequest {
  tokenId: string;
  deviceUsage?: DeviceUsage;
}
