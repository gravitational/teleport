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

// ExcludesFalse is a method that can be used instead of [].filter(Boolean)
// this removes the false values from the array type
import { PaginatedResource } from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb';
import { Server } from 'gen-proto-ts/teleport/lib/teleterm/v1/server_pb';
import { Database } from 'gen-proto-ts/teleport/lib/teleterm/v1/database_pb';
import { App } from 'gen-proto-ts/teleport/lib/teleterm/v1/app_pb';
import { Kube } from 'gen-proto-ts/teleport/lib/teleterm/v1/kube_pb';

import {
  PtyClientEvent,
  PtyEventData,
  PtyEventExit,
  PtyEventResize,
  PtyEventStart,
  PtyEventStartError,
  PtyServerEvent,
} from 'teleterm/sharedProcess/api/protogen/ptyHostService_pb';
import {
  ReloginRequest,
  SendNotificationRequest,
  CannotProxyGatewayConnection,
  GatewayCertExpired,
} from 'teleterm/services/tshdEvents';

export function resourceOneOfIsServer(
  resource: PaginatedResource['resource']
): resource is {
  oneofKind: 'server';
  server: Server;
} {
  return resource.oneofKind === 'server';
}

export function resourceOneOfIsDatabase(
  resource: PaginatedResource['resource']
): resource is {
  oneofKind: 'database';
  database: Database;
} {
  return resource.oneofKind === 'database';
}

export function resourceOneOfIsApp(
  resource: PaginatedResource['resource']
): resource is {
  oneofKind: 'app';
  app: App;
} {
  return resource.oneofKind === 'app';
}

export function resourceOneOfIsKube(
  resource: PaginatedResource['resource']
): resource is {
  oneofKind: 'kube';
  kube: Kube;
} {
  return resource.oneofKind === 'kube';
}

export function ptyEventOneOfIsStart(
  event: PtyClientEvent['event'] | PtyServerEvent['event']
): event is {
  oneofKind: 'start';
  start: PtyEventStart;
} {
  return event.oneofKind === 'start';
}

export function ptyEventOneOfIsData(
  event: PtyClientEvent['event'] | PtyServerEvent['event']
): event is {
  oneofKind: 'data';
  data: PtyEventData;
} {
  return event.oneofKind === 'data';
}

export function ptyEventOneOfIsResize(
  event: PtyClientEvent['event'] | PtyServerEvent['event']
): event is {
  oneofKind: 'resize';
  resize: PtyEventResize;
} {
  return event.oneofKind === 'resize';
}

export function ptyEventOneOfIsExit(
  event: PtyClientEvent['event'] | PtyServerEvent['event']
): event is {
  oneofKind: 'exit';
  exit: PtyEventExit;
} {
  return event.oneofKind === 'exit';
}

export function ptyEventOneOfIsStartError(
  event: PtyClientEvent['event'] | PtyServerEvent['event']
): event is {
  oneofKind: 'startError';
  startError: PtyEventStartError;
} {
  return event.oneofKind === 'startError';
}

export function notificationRequestOneOfIsCannotProxyGatewayConnection(
  subject: SendNotificationRequest['subject']
): subject is {
  oneofKind: 'cannotProxyGatewayConnection';
  cannotProxyGatewayConnection: CannotProxyGatewayConnection;
} {
  return subject.oneofKind === 'cannotProxyGatewayConnection';
}

export function reloginReasonOneOfIsGatewayCertExpired(
  reason: ReloginRequest['reason']
): reason is {
  oneofKind: 'gatewayCertExpired';
  gatewayCertExpired: GatewayCertExpired;
} {
  return reason.oneofKind === 'gatewayCertExpired';
}
