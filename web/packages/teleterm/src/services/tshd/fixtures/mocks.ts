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

import {
  makeApp,
  makeAppGateway,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';
import { getDefaultUnifiedResourcePreferences } from 'teleterm/ui/services/workspacesService';

import { MockedUnaryCall } from '../cloneableClient';
import { TshdClient, VnetClient } from '../createClient';

export class MockTshClient implements TshdClient {
  listRootClusters = () => new MockedUnaryCall({ clusters: [] });
  listLeafClusters = () => new MockedUnaryCall({ clusters: [] });
  listDatabaseUsers = () =>
    new MockedUnaryCall({
      users: [],
      totalCount: 0,
      startKey: '',
    });
  getRequestableRoles = () =>
    new MockedUnaryCall({
      roles: [],
      applicableRoles: [],
    });
  getServers = () =>
    new MockedUnaryCall({
      agents: [],
      totalCount: 0,
      startKey: '',
    });
  assumeRole = () => new MockedUnaryCall({});
  deleteAccessRequest = () => new MockedUnaryCall({});
  getAccessRequests = () =>
    new MockedUnaryCall({
      requests: [],
      totalCount: 0,
      startKey: '',
    });
  getAccessRequest = () => new MockedUnaryCall({});
  reviewAccessRequest = () => new MockedUnaryCall({});
  createAccessRequest = () => new MockedUnaryCall({});
  addCluster = () => new MockedUnaryCall(makeRootCluster());
  listGateways = () => new MockedUnaryCall({ gateways: [] });
  createGateway = () => new MockedUnaryCall(makeAppGateway());
  removeGateway = () => new MockedUnaryCall({});
  setGatewayTargetSubresourceName = () => new MockedUnaryCall(makeAppGateway());
  setGatewayLocalPort = () => new MockedUnaryCall(makeAppGateway());
  getCluster = () => new MockedUnaryCall(makeRootCluster());
  getAuthSettings = () =>
    new MockedUnaryCall({
      localAuthEnabled: true,
      authProviders: [],
      hasMessageOfTheDay: false,
      authType: 'local',
      allowPasswordless: false,
      localConnectorName: '',
    });
  removeCluster = () => new MockedUnaryCall({});
  login = () => new MockedUnaryCall({});
  loginPasswordless = undefined;
  logout = () => new MockedUnaryCall({});
  transferFile = undefined;
  reportUsageEvent = () => new MockedUnaryCall({});
  createConnectMyComputerRole = () =>
    new MockedUnaryCall({ certsReloaded: true });
  createConnectMyComputerNodeToken = () =>
    new MockedUnaryCall({ token: 'abc', labelsList: [] });
  waitForConnectMyComputerNodeJoin = () => new MockedUnaryCall({});
  updateHeadlessAuthenticationState = () => new MockedUnaryCall({});
  deleteConnectMyComputerNode = () => new MockedUnaryCall({});
  getConnectMyComputerNodeName = () => new MockedUnaryCall({ name: '' });
  listUnifiedResources = () =>
    new MockedUnaryCall({ resources: [], nextKey: '' });
  listKubernetesResources = () =>
    new MockedUnaryCall({ resources: [], nextKey: '' });
  getUserPreferences = () =>
    new MockedUnaryCall({
      userPreferences: {
        unifiedResourcePreferences: getDefaultUnifiedResourcePreferences(),
        clusterPreferences: { pinnedResources: { resourceIds: [] } },
      },
    });
  updateUserPreferences = () => new MockedUnaryCall({});
  getSuggestedAccessLists = () => new MockedUnaryCall({ accessLists: [] });
  promoteAccessRequest = () => new MockedUnaryCall({});
  updateTshdEventsServerAddress = () => new MockedUnaryCall({});
  authenticateWebDevice = () =>
    new MockedUnaryCall({
      confirmationToken: {
        id: '123456789',
        token: '7c8e7438-abe1-4cbc-b3e6-bd233bba967c',
      },
    });
  startHeadlessWatcher = () => new MockedUnaryCall({});
  getApp = () => new MockedUnaryCall({ app: makeApp() });
}

export class MockVnetClient implements VnetClient {
  start = () => new MockedUnaryCall({});
  stop = () => new MockedUnaryCall({});
  listDNSZones = () => new MockedUnaryCall({ dnsZones: [] });
  getBackgroundItemStatus = () => new MockedUnaryCall({ status: 0 });
  runDiagnostics = () => new MockedUnaryCall({});
}
