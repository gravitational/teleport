/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import {
  AuthSettings,
  AccessRequest,
  Cluster,
  CreateAccessRequestParams,
  CreateGatewayParams,
  Gateway,
  GetDatabasesResponse,
  GetKubesResponse,
  GetRequestableRolesParams,
  GetServersResponse,
  LoginLocalParams,
  LoginPasswordlessParams,
  LoginSsoParams,
  ReviewAccessRequestParams,
  ServerSideParams,
  TshAbortController,
  TshAbortSignal,
  TshClient,
  GetRequestableRolesResponse,
} from '../types';

export class MockTshClient implements TshClient {
  listRootClusters: () => Promise<Cluster[]>;
  listLeafClusters: (clusterUri: string) => Promise<Cluster[]>;
  getKubes: (params: ServerSideParams) => Promise<GetKubesResponse>;
  getDatabases: (params: ServerSideParams) => Promise<GetDatabasesResponse>;
  listDatabaseUsers: (dbUri: string) => Promise<string[]>;
  getRequestableRoles: (
    params: GetRequestableRolesParams
  ) => Promise<GetRequestableRolesResponse>;
  getServers: (params: ServerSideParams) => Promise<GetServersResponse>;
  assumeRole: (
    clusterUri: string,
    requestIds: string[],
    dropIds: string[]
  ) => Promise<void>;
  deleteAccessRequest: (clusterUri: string, requestId: string) => Promise<void>;
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
  createAbortController: () => TshAbortController;
  addRootCluster: (addr: string) => Promise<Cluster>;

  listGateways: () => Promise<Gateway[]>;
  createGateway: (params: CreateGatewayParams) => Promise<Gateway>;
  removeGateway: (gatewayUri: string) => Promise<undefined>;
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
  removeCluster = () => Promise.resolve();
  loginLocal: (
    params: LoginLocalParams,
    abortSignal?: TshAbortSignal
  ) => Promise<undefined>;
  loginSso: (
    params: LoginSsoParams,
    abortSignal?: TshAbortSignal
  ) => Promise<undefined>;
  loginPasswordless: (
    params: LoginPasswordlessParams,
    abortSignal?: TshAbortSignal
  ) => Promise<undefined>;
  logout = () => Promise.resolve();
  transferFile: () => undefined;
  reportUsageEvent: () => undefined;
}
