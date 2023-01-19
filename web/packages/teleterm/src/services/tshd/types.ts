import { ResourceKind } from 'e-teleterm/ui/DocumentAccessRequests/NewRequest/useNewRequest';
import { RequestState } from 'e-teleport/services/workflow';
import { SortType } from 'design/DataTable/types';
import { FileTransferListeners } from 'shared/components/FileTransfer';

import * as uri from 'teleterm/ui/uri';

import apiCluster from './v1/cluster_pb';
import apiDb from './v1/database_pb';
import apigateway from './v1/gateway_pb';
import apiServer from './v1/server_pb';
import apiKube from './v1/kube_pb';
import apiApp from './v1/app_pb';
import apiService from './v1/service_pb';
import apiAuthSettings from './v1/auth_settings_pb';
import apiAccessRequest from './v1/access_request_pb';
import apiUsageEvents from './v1/usage_events_pb';

export type Application = apiApp.App.AsObject;

export interface Kube extends apiKube.Kube.AsObject {
  uri: uri.KubeUri;
}

export interface Server extends apiServer.Server.AsObject {
  uri: uri.ServerUri;
}

export interface Gateway extends apigateway.Gateway.AsObject {
  uri: uri.GatewayUri;
  targetUri: uri.DatabaseUri;
}

export type AccessRequest = apiAccessRequest.AccessRequest.AsObject;
export type ResourceId = apiAccessRequest.ResourceID.AsObject;
export type AccessRequestReview = apiAccessRequest.AccessRequestReview.AsObject;

export interface GetServersResponse
  extends apiService.GetServersResponse.AsObject {
  agentsList: Server[];
}

export interface GetDatabasesResponse
  extends apiService.GetDatabasesResponse.AsObject {
  agentsList: Database[];
}

export interface GetKubesResponse extends apiService.GetKubesResponse.AsObject {
  agentsList: Kube[];
}

export type GetRequestableRolesResponse =
  apiService.GetRequestableRolesResponse.AsObject;

export type ReportUsageEventRequest = Modify<
  apiUsageEvents.ReportUsageEventRequest.AsObject,
  {
    prehogReq: Modify<
      apiUsageEvents.ReportUsageEventRequest.AsObject['prehogReq'],
      { timestamp: Date }
    >;
  }
>;

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

export interface Database extends apiDb.Database.AsObject {
  uri: uri.DatabaseUri;
}

export interface Cluster extends apiCluster.Cluster.AsObject {
  uri: uri.ClusterUri;
  loggedInUser?: LoggedInUser;
}

export type LoggedInUser = apiCluster.LoggedInUser.AsObject & {
  assumedRequests?: Record<string, AssumedRequest>;
};
export type AuthProvider = apiAuthSettings.AuthProvider.AsObject;
export type AuthSettings = apiAuthSettings.AuthSettings.AsObject;

export interface FileTransferRequest
  extends apiService.FileTransferRequest.AsObject {
  clusterUri: uri.ClusterUri;
}

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
  listLeafClusters: (clusterUri: uri.RootClusterUri) => Promise<Cluster[]>;
  listApps: (clusterUri: uri.ClusterUri) => Promise<Application[]>;
  getAllKubes: (clusterUri: uri.ClusterUri) => Promise<Kube[]>;
  getKubes: (params: ServerSideParams) => Promise<GetKubesResponse>;
  getAllDatabases: (clusterUri: uri.ClusterUri) => Promise<Database[]>;
  getDatabases: (params: ServerSideParams) => Promise<GetDatabasesResponse>;
  listDatabaseUsers: (dbUri: uri.DatabaseUri) => Promise<string[]>;
  getAllServers: (clusterUri: uri.ClusterUri) => Promise<Server[]>;
  assumeRole: (
    clusterUri: uri.RootClusterUri,
    requestIds: string[],
    dropIds: string[]
  ) => Promise<void>;
  getRequestableRoles: (
    params: GetRequestableRolesParams
  ) => Promise<GetRequestableRolesResponse>;
  getServers: (params: ServerSideParams) => Promise<GetServersResponse>;
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
  createAbortController: () => TshAbortController;
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
  logout: (clusterUri: uri.RootClusterUri) => Promise<void>;
  transferFile: (
    options: FileTransferRequest,
    abortSignal?: TshAbortSignal
  ) => FileTransferListeners;
  reportUsageEvent: (event: ReportUsageEventRequest) => Promise<void>;
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
  targetUri: uri.DatabaseUri;
  port?: string;
  user: string;
  subresource_name?: string;
};

export type ServerSideParams = {
  clusterUri: uri.ClusterUri;
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
  rootClusterUri: uri.RootClusterUri;
  reason: string;
  roles: string[];
  suggestedReviewers: string[];
  resourceIds: { kind: ResourceKind; clusterName: string; id: string }[];
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

// Replaces object property with a new type
type Modify<T, R> = Omit<T, keyof R> & R;
