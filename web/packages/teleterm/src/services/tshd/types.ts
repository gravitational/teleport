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

/* eslint-disable @typescript-eslint/ban-ts-comment*/
// @ts-ignore
import { ResourceKind } from 'e-teleterm/ui/DocumentAccessRequests/NewRequest/useNewRequest';
// @ts-ignore
import { RequestState } from 'e-teleport/services/workflow';
import { SortType } from 'design/DataTable/types';
import { FileTransferListeners } from 'shared/components/FileTransfer';
import { NodeSubKind } from 'shared/services';
import { Timestamp } from 'gen-proto-ts/google/protobuf/timestamp_pb';
import * as apiCluster from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';
import * as apiDb from 'gen-proto-ts/teleport/lib/teleterm/v1/database_pb';
import * as apiGateway from 'gen-proto-ts/teleport/lib/teleterm/v1/gateway_pb';
import * as apiServer from 'gen-proto-ts/teleport/lib/teleterm/v1/server_pb';
import * as apiKube from 'gen-proto-ts/teleport/lib/teleterm/v1/kube_pb';
import * as apiApp from 'gen-proto-ts/teleport/lib/teleterm/v1/app_pb';
import * as apiLabel from 'gen-proto-ts/teleport/lib/teleterm/v1/label_pb';
import * as apiService from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb';
import * as apiAuthSettings from 'gen-proto-ts/teleport/lib/teleterm/v1/auth_settings_pb';
import * as apiAccessRequest from 'gen-proto-ts/teleport/lib/teleterm/v1/access_request_pb';
import * as apiUsageEvents from 'gen-proto-ts/teleport/lib/teleterm/v1/usage_events_pb';
import * as apiAccessList from 'gen-proto-ts/teleport/accesslist/v1/accesslist_pb';

import * as uri from 'teleterm/ui/uri';

import {
  CloneableAbortSignal,
  CloneableRpcOptions,
  CloneableClient,
} from './cloneableClient';

// We want to reexport both the type and the value of UserType. Because it's in a namespace, we have
// to alias it first to do the reexport.
// https://www.typescriptlang.org/docs/handbook/namespaces.html#aliases
import UserType = apiCluster.LoggedInUser_UserType;

import type { ITerminalServiceClient } from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb.client';

export { UserType };
export type { CloneableAbortSignal, CloneableRpcOptions };

export interface Kube extends apiKube.Kube {
  uri: uri.KubeUri;
}

export interface Server extends apiServer.Server {
  uri: uri.ServerUri;
  subKind: NodeSubKind;
}

export interface App extends apiApp.App {
  uri: uri.AppUri;
}

export interface Gateway extends apiGateway.Gateway {
  uri: uri.GatewayUri;
  targetUri: uri.GatewayTargetUri;
  gatewayCliCommand: GatewayCLICommand;
}

export type GatewayCLICommand = apiGateway.GatewayCLICommand;

export type AccessRequest = apiAccessRequest.AccessRequest;
export type ResourceId = apiAccessRequest.ResourceID;
export type AccessRequestReview = apiAccessRequest.AccessRequestReview;
export type AccessList = apiAccessList.AccessList;

export interface GetServersResponse extends apiService.GetServersResponse {
  agents: Server[];
}

export interface GetDatabasesResponse extends apiService.GetDatabasesResponse {
  agents: Database[];
}

export interface GetKubesResponse extends apiService.GetKubesResponse {
  agents: Kube[];
}

export interface GetAppsResponse extends apiService.GetAppsResponse {
  agents: App[];
}

export type GetRequestableRolesResponse =
  apiService.GetRequestableRolesResponse;

export type ReportUsageEventRequest = apiUsageEvents.ReportUsageEventRequest;

// Available types are listed here:
// https://github.com/gravitational/teleport/blob/v9.0.3/lib/defaults/defaults.go#L513-L530
//
// The list below can get out of sync with what tsh actually implements.
export type GatewayProtocol =
  | 'postgres'
  | 'mysql'
  | 'mongodb'
  | 'cockroachdb'
  | 'redis'
  | 'sqlserver';

export interface Database extends apiDb.Database {
  uri: uri.DatabaseUri;
}

export interface Cluster extends apiCluster.Cluster {
  uri: uri.ClusterUri;
  loggedInUser?: LoggedInUser;
}

export type LoggedInUser = apiCluster.LoggedInUser & {
  assumedRequests?: Record<string, AssumedRequest>;
};
export type AuthProvider = apiAuthSettings.AuthProvider;
export type AuthSettings = apiAuthSettings.AuthSettings;

export interface FileTransferRequest extends apiService.FileTransferRequest {
  serverUri: uri.ServerUri;
}

export type WebauthnCredentialInfo = apiService.CredentialInfo;
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
  data: { credentials: WebauthnCredentialInfo[] };
  onUserResponse(index: number): void;
};
export type LoginPasswordlessRequest =
  Partial<apiService.LoginPasswordlessRequest>;

export type TshdClient = {
  listRootClusters: (abortSignal?: CloneableAbortSignal) => Promise<Cluster[]>;
  listLeafClusters: (clusterUri: uri.RootClusterUri) => Promise<Cluster[]>;
  getKubes: (params: GetResourcesParams) => Promise<GetKubesResponse>;
  getApps: (params: GetResourcesParams) => Promise<GetAppsResponse>;
  getDatabases: (params: GetResourcesParams) => Promise<GetDatabasesResponse>;
  listDatabaseUsers: (dbUri: uri.DatabaseUri) => Promise<string[]>;
  assumeRole: (
    clusterUri: uri.RootClusterUri,
    requestIds: string[],
    dropIds: string[]
  ) => Promise<void>;
  getRequestableRoles: (
    params: GetRequestableRolesParams
  ) => Promise<GetRequestableRolesResponse>;
  getServers: (params: GetResourcesParams) => Promise<GetServersResponse>;
  getAccessRequests: (
    clusterUri: uri.RootClusterUri
  ) => Promise<AccessRequest[]>;
  getAccessRequest: (
    clusterUri: uri.RootClusterUri,
    requestId: string
  ) => Promise<AccessRequest>;
  reviewAccessRequest: (
    clusterUri: uri.RootClusterUri,
    params: ReviewAccessRequestParams
  ) => Promise<AccessRequest>;
  createAccessRequest: (
    params: CreateAccessRequestParams
  ) => Promise<AccessRequest>;
  deleteAccessRequest: (
    clusterUri: uri.RootClusterUri,
    requestId: string
  ) => Promise<void>;
  addRootCluster: (addr: string) => Promise<Cluster>;

  listGateways: () => Promise<Gateway[]>;
  createGateway: (params: CreateGatewayParams) => Promise<Gateway>;
  removeGateway: (gatewayUri: uri.GatewayUri) => Promise<void>;
  setGatewayTargetSubresourceName: (
    gatewayUri: uri.GatewayUri,
    targetSubresourceName: string
  ) => Promise<Gateway>;
  setGatewayLocalPort: (
    gatewayUri: uri.GatewayUri,
    localPort: string
  ) => Promise<Gateway>;

  getCluster: (clusterUri: uri.RootClusterUri) => Promise<Cluster>;
  getAuthSettings: (clusterUri: uri.RootClusterUri) => Promise<AuthSettings>;
  removeCluster: (clusterUri: uri.RootClusterUri) => Promise<void>;
  loginLocal: (
    params: LoginLocalParams,
    abortSignal?: CloneableAbortSignal
  ) => Promise<void>;
  loginSso: (
    params: LoginSsoParams,
    abortSignal?: CloneableAbortSignal
  ) => Promise<void>;
  loginPasswordless: (
    params: LoginPasswordlessParams,
    abortSignal?: CloneableAbortSignal
  ) => Promise<void>;
  logout: (clusterUri: uri.RootClusterUri) => Promise<void>;
  transferFile: (
    options: FileTransferRequest,
    abortSignal?: CloneableAbortSignal
  ) => FileTransferListeners;
  reportUsageEvent: CloneableClient<ITerminalServiceClient>['reportUsageEvent'];
  createConnectMyComputerRole: (
    rootClusterUri: uri.RootClusterUri
  ) => Promise<CreateConnectMyComputerRoleResponse>;
  createConnectMyComputerNodeToken: (
    clusterUri: uri.RootClusterUri
  ) => Promise<CreateConnectMyComputerNodeTokenResponse>;
  waitForConnectMyComputerNodeJoin: (
    rootClusterUri: uri.RootClusterUri,
    abortSignal: CloneableAbortSignal
  ) => Promise<WaitForConnectMyComputerNodeJoinResponse>;
  deleteConnectMyComputerNode: (
    clusterUri: uri.RootClusterUri
  ) => Promise<void>;
  getConnectMyComputerNodeName: (uri: uri.RootClusterUri) => Promise<string>;

  updateHeadlessAuthenticationState: (
    params: UpdateHeadlessAuthenticationStateParams,
    abortSignal?: CloneableAbortSignal
  ) => Promise<void>;

  listUnifiedResources: (
    params: apiService.ListUnifiedResourcesRequest,
    abortSignal?: CloneableAbortSignal
  ) => Promise<ListUnifiedResourcesResponse>;

  getUserPreferences: (
    params: apiService.GetUserPreferencesRequest,
    abortSignal?: CloneableAbortSignal
  ) => Promise<UserPreferences>;
  updateUserPreferences: (
    params: apiService.UpdateUserPreferencesRequest,
    abortSignal?: CloneableAbortSignal
  ) => Promise<UserPreferences>;
  getSuggestedAccessLists: (
    params: apiService.GetSuggestedAccessListsRequest,
    abortSignal?: CloneableAbortSignal
  ) => Promise<AccessList[]>;
  promoteAccessRequest: (
    params: PromoteAccessRequestParams,
    abortSignal?: CloneableAbortSignal
  ) => Promise<AccessRequest>;

  updateTshdEventsServerAddress: (address: string) => Promise<void>;
};

interface LoginParamsBase {
  clusterUri: uri.RootClusterUri;
}

export interface LoginLocalParams extends LoginParamsBase {
  username: string;
  password: string;
  token?: string;
}

export interface LoginSsoParams extends LoginParamsBase {
  providerType: string;
  providerName: string;
}

export interface LoginPasswordlessParams extends LoginParamsBase {
  onPromptCallback(res: WebauthnLoginPrompt): void;
}

export type CreateGatewayParams = {
  targetUri: uri.GatewayTargetUri;
  port?: string;
  user: string;
  subresource_name?: string;
};

export type GetResourcesParams = {
  clusterUri: uri.ClusterUri;
  // sort is a required field because it has direct implications on performance of ListResources.
  sort: SortType | null;
  // limit cannot be omitted and must be greater than zero, otherwise ListResources is going to
  // return an error.
  limit: number;
  // search is used for regular search.
  search?: string;
  searchAsRoles?: string;
  startKey?: string;
  // query is used for advanced search.
  query?: string;
};

// Compatibility type to make sure teleport.e doesn't break.
// TODO(ravicious): Remove after teleterm.e is updated to use GetResourcesParams.
export type ServerSideParams = GetResourcesParams;

export type ReviewAccessRequestParams = {
  state: RequestState;
  reason: string;
  roles: string[];
  id: string;
  assumeStartTime?: Timestamp;
};

export type CreateAccessRequestParams =
  apiService.CreateAccessRequestRequest & {
    rootClusterUri: uri.RootClusterUri;
  };

export type GetRequestableRolesParams = {
  rootClusterUri: uri.RootClusterUri;
  resourceIds?: { kind: ResourceKind; clusterName: string; id: string }[];
};

export type AssumedRequest = {
  id: string;
  expires: Date;
  roles: string[];
};

export type Label = apiLabel.Label;

export type CreateConnectMyComputerRoleResponse =
  apiService.CreateConnectMyComputerRoleResponse;
export type CreateConnectMyComputerNodeTokenResponse =
  apiService.CreateConnectMyComputerNodeTokenResponse;
export type WaitForConnectMyComputerNodeJoinResponse =
  apiService.WaitForConnectMyComputerNodeJoinResponse & {
    server: Server;
  };

export type ListUnifiedResourcesRequest =
  apiService.ListUnifiedResourcesRequest;
export type ListUnifiedResourcesResponse = {
  resources: UnifiedResourceResponse[];
  nextKey: string;
};
export type UnifiedResourceResponse =
  | { kind: 'server'; resource: Server }
  | {
      kind: 'database';
      resource: Database;
    }
  | { kind: 'kube'; resource: Kube }
  | { kind: 'app'; resource: App };

export type UserPreferences = apiService.UserPreferences;
export type PromoteAccessRequestParams =
  apiService.PromoteAccessRequestRequest & {
    rootClusterUri: uri.RootClusterUri;
  };

export type UpdateHeadlessAuthenticationStateParams = {
  rootClusterUri: uri.RootClusterUri;
  headlessAuthenticationId: string;
  state: apiService.HeadlessAuthenticationState;
};
