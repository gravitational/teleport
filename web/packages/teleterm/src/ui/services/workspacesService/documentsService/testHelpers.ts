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
  makeAppGateway,
  makeDatabaseGateway,
  makeKubeGateway,
  makeRootCluster,
  makeServer,
} from 'teleterm/services/tshd/testHelpers';

import {
  DocumentAccessRequests,
  DocumentAuthorizeWebSession,
  DocumentCluster,
  DocumentConnectMyComputer,
  DocumentGateway,
  DocumentGatewayCliClient,
  DocumentGatewayKube,
  DocumentPtySession,
  DocumentTshNodeWithServerId,
} from './types';

export function makeDocumentCluster(
  props?: Partial<DocumentCluster>
): DocumentCluster {
  return {
    kind: 'doc.cluster',
    uri: '/docs/cluster',
    title: 'teleport-ent.asteroid.earth',
    clusterUri: makeRootCluster().uri,
    queryParams: {
      sort: {
        fieldName: 'name',
        dir: 'ASC',
      },
      resourceKinds: [],
      search: '',
      advancedSearchEnabled: false,
    },
    ...props,
  };
}

export function makeDocumentGatewayDatabase(
  props?: Partial<DocumentGateway>
): DocumentGateway {
  const gw = makeDatabaseGateway();
  return {
    kind: 'doc.gateway',
    uri: '/docs/gateway_database',
    gatewayUri: '/gateways/db-gateway',
    title: 'aurora (sre)',
    targetUri: gw.targetUri,
    port: gw.localPort,
    targetName: gw.targetName,
    targetUser: gw.targetUser,
    status: '',
    targetSubresourceName: gw.targetSubresourceName,
    origin: 'connection_list',
    ...props,
  };
}

export function makeDocumentGatewayApp(
  props?: Partial<DocumentGateway>
): DocumentGateway {
  const gw = makeAppGateway();
  return {
    kind: 'doc.gateway',
    uri: '/docs/gateway_app',
    title: 'grafana',
    targetUri: gw.targetUri,
    gatewayUri: gw.uri,
    port: gw.localPort,
    targetName: gw.targetName,
    targetUser: gw.targetUser,
    status: '',
    targetSubresourceName: gw.targetSubresourceName,
    origin: 'connection_list',
    ...props,
  };
}

export function makeDocumentPtySession(
  props?: Partial<DocumentPtySession>
): DocumentPtySession {
  return {
    kind: 'doc.terminal_shell',
    uri: '/docs/terminal_shell',
    title: '/Users/alice/Documents',
    rootClusterId: 'teleport-local',
    ...props,
  };
}

export function makeDocumentTshNode(
  props?: Partial<DocumentTshNodeWithServerId>
): DocumentTshNodeWithServerId {
  return {
    kind: 'doc.terminal_tsh_node',
    uri: '/docs/terminal_tsh_node',
    title: 'alice@node',
    serverUri: makeServer().uri,
    status: '',
    rootClusterId: 'teleport-local',
    leafClusterId: '',
    origin: 'connection_list',
    serverId: '1234abcd-1234-abcd-1234-abcd1234abcd',
    ...props,
  };
}

export function makeDocumentGatewayCliClient(
  props?: Partial<DocumentGatewayCliClient>
): DocumentGatewayCliClient {
  const gw = makeDatabaseGateway();
  return {
    kind: 'doc.gateway_cli_client',
    uri: '/docs/gateway_cli_client',
    title: 'psql · aurora (sre)',
    rootClusterId: 'teleport-local',
    leafClusterId: '',
    targetProtocol: gw.protocol,
    targetUri: gw.targetUri,
    targetName: gw.targetName,
    targetUser: gw.targetUser,
    status: '',
    ...props,
  };
}

export function makeDocumentGatewayKube(
  props?: Partial<DocumentGatewayKube>
): DocumentGatewayKube {
  const gw = makeKubeGateway();
  return {
    kind: 'doc.gateway_kube',
    uri: '/docs/gateway_kube',
    title: 'cookie',
    rootClusterId: 'teleport-local',
    leafClusterId: '',
    targetUri: gw.targetUri,
    status: '',
    origin: 'connection_list',
    ...props,
  };
}

export function makeDocumentAccessRequests(
  props?: Partial<DocumentAccessRequests>
): DocumentAccessRequests {
  return {
    kind: 'doc.access_requests',
    uri: '/docs/access_requests',
    title: 'Access Requests',
    clusterUri: makeRootCluster().uri,
    state: 'browsing',
    requestId: '1231',
    ...props,
  };
}

export function makeDocumentConnectMyComputer(
  props?: Partial<DocumentConnectMyComputer>
): DocumentConnectMyComputer {
  return {
    kind: 'doc.connect_my_computer',
    uri: '/docs/connect-my-computer',
    rootClusterUri: makeRootCluster().uri,
    status: '',
    title: 'Connect My Computer',
    ...props,
  };
}

export function makeDocumentAuthorizeWebSession(
  props?: Partial<DocumentAuthorizeWebSession>
): DocumentAuthorizeWebSession {
  return {
    kind: 'doc.authorize_web_session',
    uri: '/docs/authorize-web-session',
    title: 'Authorize Web Session',
    rootClusterUri: makeRootCluster().uri,
    webSessionRequest: {
      id: '123',
      username: 'alice',
      token: 'secret-token',
      redirectUri: '',
    },
    ...props,
  };
}
