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

import * as shared from 'shared/services/types';

import * as tsh from 'teleterm/services/tshd/types';
import * as uri from 'teleterm/ui/uri';

export type PreferredMfaType = shared.PreferredMfaType;
export type Auth2faType = shared.Auth2faType;
export type AuthProviderType = shared.AuthProviderType;
export type AuthType = shared.AuthType;

export type AuthProvider = tsh.AuthProvider;

export interface LoginLocalParams {
  kind: 'local';
  clusterUri: uri.RootClusterUri;
  username: string;
  password: string;
  token?: string;
}

export interface LoginSsoParams {
  kind: 'sso';
  clusterUri: uri.RootClusterUri;
  providerType: string;
  providerName: string;
}

export interface LoginPasswordlessParams {
  kind: 'passwordless';
  clusterUri: uri.RootClusterUri;
  onPromptCallback(res: WebauthnLoginPrompt): void;
}

export type LoginParams =
  | LoginLocalParams
  | LoginPasswordlessParams
  | LoginSsoParams;

export type LoginPasswordlessRequest = tsh.LoginPasswordlessRequest;

export type WebauthnLoginPrompt =
  | WebauthnLoginTapPrompt
  | WebauthnLoginRetapPrompt
  | WebauthnLoginPinPrompt
  | WebauthnLoginCredentialPrompt;
export type WebauthnLoginTapPrompt = { type: 'tap' };
export type WebauthnLoginRetapPrompt = { type: 'retap' };
export type WebauthnLoginPinPrompt = {
  type: 'pin';
  onUserResponse(pin: string): void;
};
export type WebauthnLoginCredentialPrompt = {
  type: 'credential';
  data: { credentials: tsh.CredentialInfo[] };
  onUserResponse(index: number): void;
};

export interface AuthSettings extends tsh.AuthSettings {
  secondFactor: Auth2faType;
  preferredMfa: PreferredMfaType;
  authType: AuthType;
  allowPasswordless: boolean;
  localConnectorName: string;
}

export type ClustersServiceState = {
  clusters: Map<
    uri.ClusterUri,
    tsh.Cluster & {
      // TODO(gzdunek): Remove assumedRequests from loggedInUser.
      // The AssumedRequest objects are needed only in AssumedRolesBar.
      // We should be able to move fetching them there.
      loggedInUser?: tsh.LoggedInUser & {
        assumedRequests?: Record<string, tsh.AssumedRequest>;
      };
    }
  >;
  gateways: Map<uri.GatewayUri, tsh.Gateway>;
};
