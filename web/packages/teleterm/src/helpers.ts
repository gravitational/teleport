// ExcludesFalse is a method that can be used instead of [].filter(Boolean)
// this removes the false values from the array type
import { PaginatedResource } from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb';
import { Server } from 'gen-proto-ts/teleport/lib/teleterm/v1/server_pb';
import { Database } from 'gen-proto-ts/teleport/lib/teleterm/v1/database_pb';
import { App } from 'gen-proto-ts/teleport/lib/teleterm/v1/app_pb';
import { Kube } from 'gen-proto-ts/teleport/lib/teleterm/v1/kube_pb';

import * as prehog from 'gen-proto-ts/prehog/v1alpha/connect_pb';

import {
  PtyClientEvent,
  PtyEventData,
  PtyEventExit,
  PtyEventResize,
  PtyEventStart,
  PtyEventStartError,
  PtyServerEvent,
} from 'teleterm/sharedProcess/api/protogen/ptyHostService_pb';

export const ExcludesFalse = Boolean as any as <T>(x: T | false) => x is T;

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
  return event.oneofKind === 'exit';
}

export function connectEventOneOfIsClusterLogin(
  event: prehog.SubmitConnectEventRequest['event']
): event is {
  oneofKind: 'clusterLogin';
  clusterLogin: prehog.ConnectClusterLoginEvent;
} {
  return event.oneofKind === 'clusterLogin';
}

export function connectEventOneOfIsProtocolUse(
  event: prehog.SubmitConnectEventRequest['event']
): event is {
  oneofKind: 'protocolUse';
  protocolUse: prehog.ConnectProtocolUseEvent;
} {
  return event.oneofKind === 'protocolUse';
}

export function connectEventOneOfIsAccessRequestCreate(
  event: prehog.SubmitConnectEventRequest['event']
): event is {
  oneofKind: 'accessRequestCreate';
  accessRequestCreate: prehog.ConnectAccessRequestCreateEvent;
} {
  return event.oneofKind === 'accessRequestCreate';
}

export function connectEventOneOfIsAccessRequestReview(
  event: prehog.SubmitConnectEventRequest['event']
): event is {
  oneofKind: 'accessRequestReview';
  accessRequestReview: prehog.ConnectAccessRequestReviewEvent;
} {
  return event.oneofKind === 'accessRequestReview';
}

export function connectEventOneOfIsAccessRequestAssumeRole(
  event: prehog.SubmitConnectEventRequest['event']
): event is {
  oneofKind: 'accessRequestAssumeRole';
  accessRequestAssumeRole: prehog.ConnectAccessRequestAssumeRoleEvent;
} {
  return event.oneofKind === 'accessRequestAssumeRole';
}

export function connectEventOneOfIsFileTransferRun(
  event: prehog.SubmitConnectEventRequest['event']
): event is {
  oneofKind: 'fileTransferRun';
  fileTransferRun: prehog.ConnectFileTransferRunEvent;
} {
  return event.oneofKind === 'fileTransferRun';
}

export function connectEventOneOfIsUserJobRoleUpdate(
  event: prehog.SubmitConnectEventRequest['event']
): event is {
  oneofKind: 'userJobRoleUpdate';
  userJobRoleUpdate: prehog.ConnectUserJobRoleUpdateEvent;
} {
  return event.oneofKind === 'userJobRoleUpdate';
}

export function connectEventOneOfIsConnectMyComputerSetup(
  event: prehog.SubmitConnectEventRequest['event']
): event is {
  oneofKind: 'connectMyComputerSetup';
  connectMyComputerSetup: prehog.ConnectConnectMyComputerSetup;
} {
  return event.oneofKind === 'connectMyComputerSetup';
}

export function connectEventOneOfIsConnectMyComputerAgentStart(
  event: prehog.SubmitConnectEventRequest['event']
): event is {
  oneofKind: 'connectMyComputerAgentStart';
  connectMyComputerAgentStart: prehog.ConnectConnectMyComputerAgentStart;
} {
  return event.oneofKind === 'connectMyComputerAgentStart';
}
