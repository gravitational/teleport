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
  makeServer,
  rootClusterUri,
  windowsDesktopUri,
} from 'teleterm/services/tshd/testHelpers';
import { makeReport } from 'teleterm/services/vnet/testHelpers';

import * as types from './types';

export function makeDocumentCluster(
  props?: Partial<types.DocumentCluster>
): types.DocumentCluster {
  return {
    kind: 'doc.cluster',
    uri: '/docs/cluster',
    title: 'teleport-ent.asteroid.earth',
    clusterUri: rootClusterUri,
    queryParams: {
      sort: {
        fieldName: 'name',
        dir: 'ASC',
      },
      resourceKinds: [],
      search: '',
      advancedSearchEnabled: false,
      statuses: [],
    },
    ...props,
  };
}

export function makeDocumentGatewayDatabase(
  props?: Partial<types.DocumentGateway>
): types.DocumentGateway {
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
  props?: Partial<types.DocumentGateway>
): types.DocumentGateway {
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
  props?: Partial<types.DocumentPtySession>
): types.DocumentPtySession {
  return {
    kind: 'doc.terminal_shell',
    uri: '/docs/terminal_shell',
    title: '/Users/alice/Documents',
    rootClusterId: 'teleport-local',
    ...props,
  };
}

export function makeDocumentTshNode(
  props?: Partial<types.DocumentTshNode>
): types.DocumentTshNode {
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
    login: 'alice',
    ...props,
  };
}

export function makeDocumentGatewayCliClient(
  props?: Partial<types.DocumentGatewayCliClient>
): types.DocumentGatewayCliClient {
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
  props?: Partial<types.DocumentGatewayKube>
): types.DocumentGatewayKube {
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
  props?: Partial<types.DocumentAccessRequests>
): types.DocumentAccessRequests {
  return {
    kind: 'doc.access_requests',
    uri: '/docs/access_requests',
    title: 'Access Requests',
    clusterUri: rootClusterUri,
    state: 'browsing',
    requestId: '1231',
    ...props,
  };
}

export function makeDocumentConnectMyComputer(
  props?: Partial<types.DocumentConnectMyComputer>
): types.DocumentConnectMyComputer {
  return {
    kind: 'doc.connect_my_computer',
    uri: '/docs/connect-my-computer',
    rootClusterUri,
    status: '',
    title: 'Connect My Computer',
    ...props,
  };
}

export function makeDocumentAuthorizeWebSession(
  props?: Partial<types.DocumentAuthorizeWebSession>
): types.DocumentAuthorizeWebSession {
  return {
    kind: 'doc.authorize_web_session',
    uri: '/docs/authorize-web-session',
    title: 'Authorize Web Session',
    rootClusterUri,
    webSessionRequest: {
      id: '123',
      username: 'alice',
      token: 'secret-token',
      redirectUri: '',
    },
    ...props,
  };
}

export function makeDocumentVnetDiagReport(
  props?: Partial<types.DocumentVnetDiagReport>
): types.DocumentVnetDiagReport {
  return {
    kind: 'doc.vnet_diag_report',
    uri: '/docs/vnet-diag-report',
    title: 'VNet Diagnostics Report',
    rootClusterUri,
    report: makeReport(),
    ...props,
  };
}

export function makeDocumentVnetInfo(
  props?: Partial<types.DocumentVnetInfo>
): types.DocumentVnetInfo {
  return {
    kind: 'doc.vnet_info',
    uri: '/docs/vnet-info',
    title: 'VNet',
    rootClusterUri,
    app: undefined,
    ...props,
  };
}

export function makeDocumentDesktopSession(
  props?: Partial<types.DocumentDesktopSession>
): types.DocumentDesktopSession {
  return {
    kind: 'doc.desktop_session',
    uri: '/docs/desktop-session',
    title: 'admin on windows-machine',
    desktopUri: windowsDesktopUri,
    login: 'admin',
    origin: 'resource_table',
    status: '',
    ...props,
  };
}
