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

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { ServerUri } from 'teleterm/ui/uri';

import * as types from '../types';

export class MockTshClient implements types.TshClient {
  listRootClusters: () => Promise<types.Cluster[]>;
  listLeafClusters = () => Promise.resolve([]);
  getKubes: (
    params: types.GetResourcesParams
  ) => Promise<types.GetKubesResponse>;
  getDatabases: (
    params: types.GetResourcesParams
  ) => Promise<types.GetDatabasesResponse>;
  listDatabaseUsers: (dbUri: string) => Promise<string[]>;
  getRequestableRoles: (
    params: types.GetRequestableRolesParams
  ) => Promise<types.GetRequestableRolesResponse>;
  getServers: (
    params: types.GetResourcesParams
  ) => Promise<types.GetServersResponse>;
  assumeRole: (
    clusterUri: string,
    requestIds: string[],
    dropIds: string[]
  ) => Promise<void>;
  deleteAccessRequest: (clusterUri: string, requestId: string) => Promise<void>;
  getAccessRequests: (clusterUri: string) => Promise<types.AccessRequest[]>;
  getAccessRequest: (
    clusterUri: string,
    requestId: string
  ) => Promise<types.AccessRequest>;
  reviewAccessRequest: (
    clusterUri: string,
    params: types.ReviewAccessRequestParams
  ) => Promise<types.AccessRequest>;
  createAccessRequest: (
    params: types.CreateAccessRequestParams
  ) => Promise<types.AccessRequest>;
  createAbortController: () => types.TshAbortController;
  addRootCluster: (addr: string) => Promise<types.Cluster>;

  listGateways: () => Promise<types.Gateway[]>;
  createGateway: (params: types.CreateGatewayParams) => Promise<types.Gateway>;
  removeGateway: (gatewayUri: string) => Promise<undefined>;
  setGatewayTargetSubresourceName: (
    gatewayUri: string,
    targetSubresourceName: string
  ) => Promise<types.Gateway>;
  setGatewayLocalPort: (
    gatewayUri: string,
    localPort: string
  ) => Promise<types.Gateway>;

  getCluster = () => Promise.resolve(makeRootCluster());
  getAuthSettings: (clusterUri: string) => Promise<types.AuthSettings>;
  removeCluster = () => Promise.resolve();
  loginLocal: (
    params: types.LoginLocalParams,
    abortSignal?: types.TshAbortSignal
  ) => Promise<undefined>;
  loginSso: (
    params: types.LoginSsoParams,
    abortSignal?: types.TshAbortSignal
  ) => Promise<undefined>;
  loginPasswordless: (
    params: types.LoginPasswordlessParams,
    abortSignal?: types.TshAbortSignal
  ) => Promise<undefined>;
  logout = () => Promise.resolve();
  transferFile: () => undefined;
  reportUsageEvent: () => undefined;

  createConnectMyComputerRole = () => Promise.resolve({ certsReloaded: true });
  createConnectMyComputerNodeToken = () =>
    Promise.resolve({ token: 'abc', labelsList: [] });
  deleteConnectMyComputerToken = () => Promise.resolve();
  waitForConnectMyComputerNodeJoin: () => Promise<types.WaitForConnectMyComputerNodeJoinResponse>;

  updateHeadlessAuthenticationState: (
    params: types.UpdateHeadlessAuthenticationStateParams
  ) => Promise<void>;
  deleteConnectMyComputerNode: () => Promise<void>;
  getConnectMyComputerNodeName = () => Promise.resolve('');
}
