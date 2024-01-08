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
  DocumentGateway,
  DocumentGatewayKube,
  DocumentTshKube,
  DocumentTshNode,
  DocumentTshNodeWithServerId,
  isDocumentTshNodeWithServerId,
  DocumentGatewayApp,
} from 'teleterm/ui/services/workspacesService';
import { unique } from 'teleterm/ui/utils/uid';

import {
  TrackedGatewayConnection,
  TrackedKubeConnection,
  TrackedServerConnection,
  TrackedAppConnection,
} from './types';

export function getGatewayConnectionByDocument(document: DocumentGateway) {
  return (i: TrackedGatewayConnection) =>
    i.kind === 'connection.gateway' &&
    i.targetUri === document.targetUri &&
    i.targetUser === document.targetUser;
}

export function getServerConnectionByDocument(document: DocumentTshNode) {
  return (i: TrackedServerConnection) =>
    isDocumentTshNodeWithServerId(document) &&
    i.kind === 'connection.server' &&
    i.serverUri === document.serverUri &&
    i.login === document.login;
}

// DELETE IN 15.0.0. See DocumentGatewayKube for more details.
export function getKubeConnectionByDocument(document: DocumentTshKube) {
  return (i: TrackedKubeConnection) =>
    i.kind === 'connection.kube' && i.kubeUri === document.kubeUri;
}

export function getGatewayKubeConnectionByDocument(
  document: DocumentGatewayKube
) {
  return (i: TrackedKubeConnection) =>
    i.kind === 'connection.kube' && i.kubeUri === document.targetUri;
}

export function getGatewayAppConnectionByDocument(
  document: DocumentGatewayApp
) {
  return (i: TrackedAppConnection) =>
    i.kind === 'connection.app' && i.targetUri === document.targetUri;
}

export function getGatewayDocumentByConnection(
  connection: TrackedGatewayConnection
) {
  return (i: DocumentGateway) =>
    i.kind === 'doc.gateway' &&
    i.targetUri === connection.targetUri &&
    i.targetUser === connection.targetUser;
}

export function getGatewayKubeDocumentByConnection(
  connection: TrackedKubeConnection
) {
  return (i: DocumentGatewayKube) =>
    i.kind === 'doc.gateway_kube' && i.targetUri === connection.kubeUri;
}

// DELETE IN 15.0.0. See DocumentGatewayKube for more details.
export function getKubeDocumentByConnection(connection: TrackedKubeConnection) {
  return (i: DocumentTshKube) =>
    i.kind === 'doc.terminal_tsh_kube' && i.kubeUri === connection.kubeUri;
}

export function getServerDocumentByConnection(
  connection: TrackedServerConnection
) {
  return (i: DocumentTshNode) =>
    i.kind === 'doc.terminal_tsh_node' &&
    isDocumentTshNodeWithServerId(i) &&
    i.serverUri === connection.serverUri &&
    i.login === connection.login;
}

export function getGatewayAppDocumentByConnection(
  connection: TrackedAppConnection
) {
  return (i: DocumentGatewayApp) =>
    i.kind === 'doc.gateway_app' && i.targetUri === connection.targetUri;
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
    gatewayUri: document.gatewayUri,
  };
}

export function createServerConnection(
  document: DocumentTshNodeWithServerId
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

export function createKubeConnection(
  document: DocumentTshKube
): TrackedKubeConnection {
  return {
    kind: 'connection.kube',
    connected: document.status === 'connected',
    id: unique(),
    title: document.title,
    kubeConfigRelativePath: document.kubeConfigRelativePath,
    kubeUri: document.kubeUri,
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

export function createGatewayAppConnection(
  document: DocumentGatewayApp
): TrackedAppConnection {
  return {
    kind: 'connection.app',
    connected: true,
    id: unique(),
    title: document.title,
    targetUri: document.targetUri,
    port: document.port,
    gatewayUri: document.gatewayUri,
  };
}
