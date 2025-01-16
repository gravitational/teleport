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
  makeDatabase,
  makeKube,
  makeRootCluster,
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
  DocumentTshNode,
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
  return {
    kind: 'doc.gateway',
    uri: '/docs/gateway_database',
    title: 'aurora (sre)',
    targetUri: makeDatabase().uri,
    gatewayUri: '/gateways/db-gateway',
    port: '1232',
    targetName: 'aurora',
    targetUser: 'sre',
    status: '',
    targetSubresourceName: '',
    origin: 'connection_list',
    ...props,
  };
}

export function makeDocumentGatewayApp(
  props?: Partial<DocumentGateway>
): DocumentGateway {
  return {
    kind: 'doc.gateway',
    uri: '/docs/gateway_app',
    title: 'grafana',
    targetUri: makeApp().uri,
    gatewayUri: '/gateways/app-gateway',
    port: '1232',
    targetName: '',
    targetUser: '',
    status: '',
    targetSubresourceName: '',
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
    ...props,
  };
}

export function makeDocumentTshNode(
  props?: Partial<DocumentTshNode>
): DocumentTshNode {
  return {
    kind: 'doc.terminal_tsh_node',
    uri: '/docs/terminal_tsh_node',
    title: 'alice@node',
    serverId: '89128321312dsf213r',
    ...props,
  };
}

export function makeDocumentGatewayCliClient(
  props?: Partial<DocumentGatewayCliClient>
): DocumentGatewayCliClient {
  return {
    kind: 'doc.gateway_cli_client',
    uri: '/docs/gateway_cli_client',
    title: 'psql · aurora (sre)',
    targetUri: makeDatabase().uri,
    gatewayUri: '/gateways/db-gateway',
    port: '1232',
    targetName: 'aurora',
    targetUser: 'sre',
    status: '',
    targetSubresourceName: '',
    origin: 'connection_list',
    ...props,
  };
}

export function makeDocumentGatewayKube(
  props?: Partial<DocumentGatewayKube>
): DocumentGatewayKube {
  return {
    kind: 'doc.gateway_kube',
    uri: '/docs/gateway_kube',
    title: 'cookie',
    targetUri: makeKube().uri,
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
    webSessionRequest: { id: '123', username: 'alice', token: 'secret-token' },
    ...props,
  };
}
