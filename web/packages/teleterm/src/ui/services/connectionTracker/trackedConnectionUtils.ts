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
  Document,
  DocumentDesktopSession,
  DocumentGateway,
  DocumentGatewayKube,
  DocumentTshNode,
  getDocumentGatewayTargetUriKind,
} from 'teleterm/ui/services/workspacesService';
import { unique } from 'teleterm/ui/utils/uid';

import {
  TrackedConnection,
  TrackedDesktopConnection,
  TrackedGatewayConnection,
  TrackedKubeConnection,
  TrackedServerConnection,
} from './types';

/*
 * Getting a connection by a document.
 */

/**
 *
 * getGatewayConnectionByDocument looks for a connection that has the same gateway params as the
 * document.
 *
 * ---
 *
 * This function is used in two scenarios. It's used when recreating the list of connections based
 * on open documents. If there's no connection found that matches DocumentGateway, a new connection
 * is added to the list.
 *
 * It's also used when opening new gateways for databases and apps to find an existing connection
 * and call it's `activate` handler, which is going to open an existing document. If no existing
 * connection is found, a new document is added to the workspace.
 */
export function getGatewayConnectionByDocument(
  document: DocumentGateway
): (c: TrackedConnection) => boolean {
  const targetKind = getDocumentGatewayTargetUriKind(document.targetUri);

  switch (targetKind) {
    case 'db': {
      return c =>
        c.kind === 'connection.gateway' &&
        c.targetUri === document.targetUri &&
        c.targetUser === document.targetUser;
    }
    case 'app': {
      return c =>
        c.kind === 'connection.gateway' &&
        c.targetUri === document.targetUri &&
        c.targetSubresourceName === document.targetSubresourceName;
    }
    default: {
      targetKind satisfies never;
    }
  }
}

export function getServerConnectionByDocument(document: DocumentTshNode) {
  return (i: TrackedServerConnection) =>
    i.kind === 'connection.server' &&
    i.serverUri === document.serverUri &&
    i.login === document.login;
}

export function getGatewayKubeConnectionByDocument(
  document: DocumentGatewayKube
) {
  return (i: TrackedKubeConnection) =>
    i.kind === 'connection.kube' && i.kubeUri === document.targetUri;
}

export function getDesktopDocumentByConnection(
  connection: TrackedDesktopConnection
): (d: Document) => boolean {
  return d =>
    d.kind === 'doc.desktop_session' &&
    d.desktopUri === connection.desktopUri &&
    d.login === connection.login;
}

export function getDesktopConnectionByDocument(
  document: DocumentDesktopSession
) {
  return (i: TrackedDesktopConnection) =>
    i.kind === 'connection.desktop' &&
    i.desktopUri === document.desktopUri &&
    i.login === document.login;
}

/*
 * Getting a document by a connection.
 */

/**
 * getGatewayDocumentByConnection looks for a DocumentGateway that has the same gateway params as
 * the connection.
 *
 * ---
 *
 * This function is used in two scenarios. It's used when activating (clicking) a connection in the
 * connections list to find a document to open if there's already a gateway for the given connection.
 *
 * The `activate` handler is also called when the user attempts to open a gateway for a database or
 * an app. That UI action first prepares a doc with provided gateway parameters. If there's a
 * connection which matches the gateway parameters from the doc (getGatewayConnectionByDocument),
 * its `activate` handler is called.
 *
 * The second scenario is when disconnecting a connection from the connections list to find a
 * document which should be closed.
 */
export function getGatewayDocumentByConnection(
  connection: TrackedGatewayConnection
): (d: Document) => boolean {
  const targetKind = getDocumentGatewayTargetUriKind(connection.targetUri);

  switch (targetKind) {
    case 'db': {
      return d =>
        d.kind === 'doc.gateway' &&
        d.targetUri === connection.targetUri &&
        d.targetUser === connection.targetUser;
    }
    case 'app': {
      return d =>
        d.kind === 'doc.gateway' &&
        d.targetUri === connection.targetUri &&
        d.targetSubresourceName === connection.targetSubresourceName;
    }
    default: {
      targetKind satisfies never;
    }
  }
}

export function getGatewayKubeDocumentByConnection(
  connection: TrackedKubeConnection
) {
  return (i: DocumentGatewayKube) =>
    i.kind === 'doc.gateway_kube' && i.targetUri === connection.kubeUri;
}

export function getServerDocumentByConnection(
  connection: TrackedServerConnection
) {
  return (i: DocumentTshNode) =>
    i.kind === 'doc.terminal_tsh_node' &&
    i.serverUri === connection.serverUri &&
    i.login === connection.login;
}

export function createGatewayConnection(
  document: DocumentGateway
): TrackedGatewayConnection {
  return {
    kind: 'connection.gateway',
    connected: true,
    id: unique(),
    title: document.title,
    port: document.port,
    targetUri: document.targetUri,
    targetUser: document.targetUser,
    targetName: document.targetName,
    targetSubresourceName: document.targetSubresourceName,
  };
}

export function createServerConnection(
  document: DocumentTshNode
): TrackedServerConnection {
  return {
    kind: 'connection.server',
    connected: document.status === 'connected',
    id: unique(),
    title: document.title,
    login: document.login,
    serverUri: document.serverUri,
  };
}

export function createGatewayKubeConnection(
  document: DocumentGatewayKube
): TrackedKubeConnection {
  return {
    kind: 'connection.kube',
    connected: true,
    id: unique(),
    title: document.title,
    kubeUri: document.targetUri,
  };
}

export function createDesktopConnection(
  document: DocumentDesktopSession
): TrackedDesktopConnection {
  return {
    kind: 'connection.desktop',
    connected: true,
    id: unique(),
    title: document.title,
    desktopUri: document.desktopUri,
    login: document.login,
  };
}
