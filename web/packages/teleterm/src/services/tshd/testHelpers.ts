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

import * as tsh from './types';

export const makeServer = (props: Partial<tsh.Server> = {}): tsh.Server => ({
  uri: '/clusters/teleport-local/servers/178ef081-259b-4aa5-a018-449b5ea7e694',
  tunnel: false,
  name: '178ef081-259b-4aa5-a018-449b5ea7e694',
  hostname: 'foo',
  addr: '127.0.0.1:3022',
  labelsList: [],
  ...props,
});

export const databaseUri = '/clusters/teleport-local/dbs/foo';
export const kubeUri = '/clusters/teleport-local/kubes/foo';

export const makeDatabase = (
  props: Partial<tsh.Database> = {}
): tsh.Database => ({
  uri: databaseUri,
  name: 'foo',
  protocol: 'postgres',
  type: 'self-hosted',
  desc: '',
  hostname: '',
  addr: '',
  labelsList: [],
  ...props,
});

export const makeKube = (props: Partial<tsh.Kube> = {}): tsh.Kube => ({
  name: 'foo',
  labelsList: [],
  uri: '/clusters/bar/kubes/foo',
  ...props,
});

export const makeLabelsList = (labels: Record<string, string>): tsh.Label[] =>
  Object.entries(labels).map(([name, value]) => ({ name, value }));

export const makeRootCluster = (
  props: Partial<tsh.Cluster> = {}
): tsh.Cluster => ({
  uri: '/clusters/teleport-local',
  name: 'teleport-local',
  connected: true,
  leaf: false,
  proxyHost: 'teleport-local:3080',
  authClusterId: '73c4746b-d956-4f16-9848-4e3469f70762',
  loggedInUser: makeLoggedInUser(),
  ...props,
});

export const makeLoggedInUser = (
  props: Partial<tsh.LoggedInUser> = {}
): tsh.LoggedInUser => ({
  activeRequestsList: [],
  assumedRequests: {},
  name: 'alice',
  acl: {
    recordedSessions: {
      list: true,
      read: true,
      edit: true,
      create: true,
      pb_delete: true,
      use: true,
    },
    activeSessions: {
      list: true,
      read: true,
      edit: true,
      create: true,
      pb_delete: true,
      use: true,
    },
    authConnectors: {
      list: true,
      read: true,
      edit: true,
      create: true,
      pb_delete: true,
      use: true,
    },
    roles: {
      list: true,
      read: true,
      edit: true,
      create: true,
      pb_delete: true,
      use: true,
    },
    users: {
      list: true,
      read: true,
      edit: true,
      create: true,
      pb_delete: true,
      use: true,
    },
    trustedClusters: {
      list: true,
      read: true,
      edit: true,
      create: true,
      pb_delete: true,
      use: true,
    },
    events: {
      list: true,
      read: true,
      edit: true,
      create: true,
      pb_delete: true,
      use: true,
    },
    tokens: {
      list: true,
      read: true,
      edit: true,
      create: true,
      pb_delete: true,
      use: true,
    },
    servers: {
      list: true,
      read: true,
      edit: true,
      create: true,
      pb_delete: true,
      use: true,
    },
    apps: {
      list: true,
      read: true,
      edit: true,
      create: true,
      pb_delete: true,
      use: true,
    },
    dbs: {
      list: true,
      read: true,
      edit: true,
      create: true,
      pb_delete: true,
      use: true,
    },
    kubeservers: {
      list: true,
      read: true,
      edit: true,
      create: true,
      pb_delete: true,
      use: true,
    },
    accessRequests: {
      list: true,
      read: true,
      edit: true,
      create: true,
      pb_delete: true,
      use: true,
    },
  },
  sshLoginsList: [],
  rolesList: [],
  requestableRolesList: [],
  suggestedReviewersList: [],
  userType: tsh.UserType.USER_TYPE_LOCAL,
  ...props,
});

export const makeDatabaseGateway = (
  props: Partial<tsh.Gateway> = {}
): tsh.Gateway => ({
  uri: '/gateways/foo',
  targetName: 'sales-production',
  targetUri: databaseUri,
  targetUser: 'alice',
  localAddress: 'localhost',
  localPort: '1337',
  protocol: 'postgres',
  gatewayCliCommand: {
    path: '/foo/psql',
    argsList: ['psql', 'localhost:1337'],
    envList: [],
    preview: 'psql localhost:1337',
  },
  targetSubresourceName: 'bar',
  ...props,
});

export const makeKubeGateway = (
  props: Partial<tsh.Gateway> = {}
): tsh.Gateway => ({
  uri: '/gateways/foo',
  targetName: 'foo',
  targetUri: kubeUri,
  targetUser: '',
  localAddress: 'localhost',
  localPort: '1337',
  protocol: '',
  gatewayCliCommand: {
    path: '/bin/kubectl',
    argsList: ['version'],
    envList: ['KUBECONFIG=/path/to/kubeconfig'],
    preview: 'KUBECONFIG=/path/to/kubeconfig /bin/kubectl version',
  },
  targetSubresourceName: '',
  ...props,
});
