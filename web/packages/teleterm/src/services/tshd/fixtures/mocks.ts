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

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';

import * as types from '../types';

export class MockTshClient implements types.TshdClient {
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
  getApps: (params: types.GetResourcesParams) => Promise<types.GetAppsResponse>;
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
  deleteConnectMyComputerNode = () => Promise.resolve();
  getConnectMyComputerNodeName = () => Promise.resolve('');

  listUnifiedResources = async () => ({ resources: [], nextKey: '' });
  getUserPreferences = async () => ({});
  updateUserPreferences = async () => ({});
  getSuggestedAccessLists = async () => [];
  promoteAccessRequest = async () => undefined;

  updateTshdEventsServerAddress: (address: string) => Promise<void>;
}
