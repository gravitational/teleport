import {
  DocumentGateway,
  DocumentTshNode,
} from 'teleterm/ui/services/workspacesService';
import { unique } from 'teleterm/ui/utils/uid';
import { TrackedGatewayConnection, TrackedServerConnection } from './types';

export function getGatewayConnectionByDocument(document: DocumentGateway) {
  return (i: TrackedGatewayConnection) =>
    i.kind === 'connection.gateway' &&
    i.targetUri === document.targetUri &&
    i.port === document.port;
}

export function getServerConnectionByDocument(document: DocumentTshNode) {
  return (i: TrackedServerConnection) =>
    i.kind === 'connection.server' &&
    i.serverUri === document.serverUri &&
    i.login === document.login;
}

export function getGatewayDocumentByConnection(
  connection: TrackedGatewayConnection
) {
  return (i: DocumentGateway) =>
    i.kind === 'doc.gateway' &&
    i.targetUri === connection.targetUri &&
    i.port === connection.port;
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
    gatewayUri: document.gatewayUri,
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
