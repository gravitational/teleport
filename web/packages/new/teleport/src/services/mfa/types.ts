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

import type { AuthProviderType } from '../../config/auth';
import type {
  Base64urlString,
  CreateNewHardwareDeviceRequest,
} from '../auth/types';

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

export interface AddNewTotpDeviceRequest {
  tokenId: string;
  deviceName: string;
  secondFactorToken: string;
}

export type AddNewHardwareDeviceRequest = CreateNewHardwareDeviceRequest & {
  deviceName: string;
};

export interface SaveNewHardwareDeviceRequest {
  addRequest: AddNewHardwareDeviceRequest;
  credential: Credential;
}

export interface MfaAuthenticateChallengeJson {
  sso_challenge?: SsoChallenge;
  totp_challenge?: boolean;
  webauthn_challenge?: {
    publicKey: PublicKeyCredentialRequestOptionsJSON;
  };
}

export interface MfaAuthenticateChallenge {
  ssoChallenge?: SsoChallenge;
  totpChallenge?: boolean;
  webauthnPublicKey?: PublicKeyCredentialRequestOptions;
}

export interface SsoChallenge {
  channelId: string;
  redirectUrl: string;
  requestId: string;
  device: {
    connectorId: string;
    connectorType: AuthProviderType;
    displayName: string;
  };
}

export interface MfaRegistrationChallengeJson {
  totp?: {
    qrCode: Base64urlString;
  };
  webauthn?: {
    publicKey: PublicKeyCredentialCreationOptionsJSON;
  };
}

export interface MfaRegistrationChallenge {
  qrCode: Base64urlString;
  webauthnPublicKey: PublicKeyCredentialCreationOptions;
}

export interface MfaChallengeResponse {
  totp_code?: string;
  webauthn_response?: WebauthnAssertionResponse;
  sso_response?: SsoChallengeResponse;
}

export interface SsoChallengeResponse {
  requestId: string;
  token: string;
}

export interface WebauthnAssertionResponse {
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
}

export interface WebauthnAttestationResponse {
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
}

// TODO(Joerger): In order to check if mfa is required for a leaf host, the leaf
// clusterID must be included in the request. Currently, only IsMfaRequiredApp
// supports this functionality.
export type IsMfaRequiredRequest =
  | IsMfaRequiredDatabase
  | IsMfaRequiredNode
  | IsMfaRequiredKube
  | IsMfaRequiredWindowsDesktop
  | IsMfaRequiredApp
  | IsMfaRequiredAdminAction;

export interface IsMfaRequiredResponse {
  required: boolean;
}

export interface IsMfaRequiredDatabase {
  database: {
    // service_name is the database service name.
    service_name: string;
    // protocol is the type of the database protocol.
    protocol: string;
    // username is an optional database username.
    username?: string;
    // database_name is an optional database name.
    database_name?: string;
  };
}

export interface IsMfaRequiredNode {
  node: {
    // node_name can be node's hostname or UUID.
    node_name: string;
    // login is the OS login name.
    login: string;
  };
}

export interface IsMfaRequiredWindowsDesktop {
  windows_desktop: {
    // desktop_name is the Windows Desktop server name.
    desktop_name: string;
    // login is the Windows desktop user login.
    login: string;
  };
}

export interface IsMfaRequiredKube {
  kube: {
    // cluster_name is the name of the kube cluster.
    cluster_name: string;
  };
}

export interface IsMfaRequiredApp {
  app: {
    // fqdn indicates (tentatively) the fully qualified domain name of the application.
    fqdn: string;
    // public_addr is the public address of the application.
    public_addr: string;
    // cluster_name is the cluster within which this application is running.
    cluster_name: string;
  };
}

export interface IsMfaRequiredAdminAction {
  // empty object.
  admin_action: Record<string, never>;
}

// MfaChallengeScope is an mfa challenge scope. Possible values are defined in mfa.proto
export enum MfaChallengeScope {
  UNSPECIFIED = 0,
  LOGIN = 1,
  PASSWORDLESS_LOGIN = 2,
  HEADLESS_LOGIN = 3,
  MANAGE_DEVICES = 4,
  ACCOUNT_RECOVERY = 5,
  USER_SESSION = 6,
  ADMIN_ACTION = 7,
  CHANGE_PASSWORD = 8,
}
