import { ResourceKind } from 'e-teleterm/ui/DocumentAccessRequests/NewRequest/useNewRequest';

import { SortType } from 'design/DataTable/types';

import { RequestState } from 'e-teleport/services/workflow';

import { FileTransferListeners } from 'shared/components/FileTransfer';

import apiCluster from './v1/cluster_pb';
import apiDb from './v1/database_pb';
import apigateway from './v1/gateway_pb';
import apiServer from './v1/server_pb';
import apiKube from './v1/kube_pb';
import apiApp from './v1/app_pb';
import apiService from './v1/service_pb';
import apiAuthSettings from './v1/auth_settings_pb';
import apiAccessRequest from './v1/access_request_pb';

export type Application = apiApp.App.AsObject;
export type Kube = apiKube.Kube.AsObject;
export type Server = apiServer.Server.AsObject;
export type Gateway = apigateway.Gateway.AsObject;
export type AccessRequest = apiAccessRequest.AccessRequest.AsObject;
export type ResourceId = apiAccessRequest.ResourceID.AsObject;
export type AccessRequestReview = apiAccessRequest.AccessRequestReview.AsObject;
export type GetServersResponse = apiService.GetServersResponse.AsObject;
export type GetDatabasesResponse = apiService.GetDatabasesResponse.AsObject;
export type GetKubesResponse = apiService.GetKubesResponse.AsObject;
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
export type Database = apiDb.Database.AsObject;
export type Cluster = Omit<apiCluster.Cluster.AsObject, 'loggedInUser'> & {
  loggedInUser?: LoggedInUser;
};
export type LoggedInUser = apiCluster.LoggedInUser.AsObject & {
  assumedRequests?: Record<string, AssumedRequest>;
};
export type AuthProvider = apiAuthSettings.AuthProvider.AsObject;
export type AuthSettings = apiAuthSettings.AuthSettings.AsObject;

export type FileTransferRequest = apiService.FileTransferRequest.AsObject;

export type WebauthnCredentialInfo = apiService.CredentialInfo.AsObject;
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
  Partial<apiService.LoginPasswordlessRequest.AsObject>;

export type TshClient = {
  listRootClusters: () => Promise<Cluster[]>;
  listLeafClusters: (clusterUri: string) => Promise<Cluster[]>;
  listApps: (clusterUri: string) => Promise<Application[]>;
  getAllKubes: (clusterUri: string) => Promise<Kube[]>;
  getKubes: (params: ServerSideParams) => Promise<GetKubesResponse>;
  getAllDatabases: (clusterUri: string) => Promise<Database[]>;
  getDatabases: (params: ServerSideParams) => Promise<GetDatabasesResponse>;
  listDatabaseUsers: (dbUri: string) => Promise<string[]>;
  getAllServers: (clusterUri: string) => Promise<Server[]>;
  assumeRole: (
    clusterUri: string,
    requestIds: string[],
    dropIds: string[]
  ) => Promise<void>;
  getRequestableRoles: (clusterUri: string) => Promise<string[]>;
  getServers: (params: ServerSideParams) => Promise<GetServersResponse>;
  getAccessRequests: (clusterUri: string) => Promise<AccessRequest[]>;
  getAccessRequest: (
    clusterUri: string,
    requestId: string
  ) => Promise<AccessRequest>;
  reviewAccessRequest: (
    clusterUri: string,
    params: ReviewAccessRequestParams
  ) => Promise<AccessRequest>;
  createAccessRequest: (
    params: CreateAccessRequestParams
  ) => Promise<AccessRequest>;
  deleteAccessRequest: (clusterUri: string, requestId: string) => Promise<void>;
  createAbortController: () => TshAbortController;
  addRootCluster: (addr: string) => Promise<Cluster>;

  listGateways: () => Promise<Gateway[]>;
  createGateway: (params: CreateGatewayParams) => Promise<Gateway>;
  removeGateway: (gatewayUri: string) => Promise<void>;
  setGatewayTargetSubresourceName: (
    gatewayUri: string,
    targetSubresourceName: string
  ) => Promise<Gateway>;
  setGatewayLocalPort: (
    gatewayUri: string,
    localPort: string
  ) => Promise<Gateway>;

  getCluster: (clusterUri: string) => Promise<Cluster>;
  getAuthSettings: (clusterUri: string) => Promise<AuthSettings>;
  removeCluster: (clusterUri: string) => Promise<void>;
  loginLocal: (
    params: LoginLocalParams,
    abortSignal?: TshAbortSignal
  ) => Promise<void>;
  loginSso: (
    params: LoginSsoParams,
    abortSignal?: TshAbortSignal
  ) => Promise<void>;
  loginPasswordless: (
    params: LoginPasswordlessParams,
    abortSignal?: TshAbortSignal
  ) => Promise<void>;
  logout: (clusterUri: string) => Promise<void>;
  transferFile: (
    options: FileTransferRequest,
    abortSignal?: TshAbortSignal
  ) => FileTransferListeners;
};

export type TshAbortController = {
  signal: TshAbortSignal;
  abort(): void;
};

export type TshAbortSignal = {
  addEventListener(cb: (...args: any[]) => void): void;
  removeEventListener(cb: (...args: any[]) => void): void;
};

interface LoginParamsBase {
  clusterUri: string;
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
  targetUri: string;
  port?: string;
  user: string;
  subresource_name?: string;
};

export type ServerSideParams = {
  clusterUri: string;
  search?: string;
  searchAsRoles?: string;
  sort?: SortType;
  startKey?: string;
  limit?: number;
  query?: string;
};

export type ReviewAccessRequestParams = {
  state: RequestState;
  reason: string;
  roles: string[];
  id: string;
};

export type CreateAccessRequestParams = {
  rootClusterUri: string;
  reason: string;
  roles: string[];
  suggestedReviewers: string[];
  resourceIds: { kind: ResourceKind; clusterName: string; id: string }[];
};

export type AssumedRequest = {
  id: string;
  expires: Date;
  roles: string[];
};
