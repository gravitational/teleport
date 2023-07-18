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

import type * as tsh from './types';

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
  loggedInUser: {
    activeRequestsList: [],
    assumedRequests: {},
    name: 'admin',
    acl: {},
    sshLoginsList: [],
    rolesList: [],
    requestableRolesList: [],
    suggestedReviewersList: [],
  },
  ...props,
});

export const makeDBGateway = (
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
