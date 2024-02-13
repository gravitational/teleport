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

import * as api from 'gen-proto-ts/teleport/lib/teleterm/v1/usage_events_pb';
import * as prehogApi from 'gen-proto-ts/prehog/v1alpha/connect_pb';

import {
  connectEventOneOfIsAccessRequestAssumeRole,
  connectEventOneOfIsAccessRequestCreate,
  connectEventOneOfIsAccessRequestReview,
  connectEventOneOfIsClusterLogin,
  connectEventOneOfIsConnectMyComputerAgentStart,
  connectEventOneOfIsConnectMyComputerSetup,
  connectEventOneOfIsFileTransferRun,
  connectEventOneOfIsProtocolUse,
} from 'teleterm/helpers';

import * as types from './types';

/**
 * Maps a plain JS object into a gRPC request object.
 */
export function mapUsageEvent(event: types.ReportUsageEventRequest) {
  return api.ReportUsageEventRequest.create({
    authClusterId: event.authClusterId,
    prehogReq: mapPrehogBody(event.prehogReq),
  });
}

function mapPrehogBody(
  plainReq: types.ReportUsageEventRequest['prehogReq']
): prehogApi.SubmitConnectEventRequest {
  if (!plainReq) {
    throw new Error(`Unrecognized event: ${JSON.stringify(plainReq)}`);
  }

  const req = prehogApi.SubmitConnectEventRequest.create({
    timestamp: plainReq.timestamp,
    distinctId: plainReq.distinctId,
  });

  // Non-anonymized events.
  if (plainReq.event.oneofKind === 'userJobRoleUpdate') {
    const event = plainReq.event.userJobRoleUpdate;
    const reqEvent = prehogApi.ConnectUserJobRoleUpdateEvent.create({
      jobRole: event.jobRole,
    });

    req.event = {
      oneofKind: 'userJobRoleUpdate',
      userJobRoleUpdate: reqEvent,
    };

    return req;
  }

  // Anonymized events.
  if (connectEventOneOfIsClusterLogin(plainReq.event)) {
    const event = plainReq.event.clusterLogin;
    const reqEvent = prehogApi.ConnectClusterLoginEvent.create({
      clusterName: event.clusterName,
      userName: event.userName,
      connectorType: event.connectorType,
      os: event.os,
      arch: event.arch,
      osVersion: event.osVersion,
      appVersion: event.appVersion,
    });

    req.event = {
      oneofKind: 'clusterLogin',
      clusterLogin: reqEvent,
    };

    return req;
  }
  if (connectEventOneOfIsProtocolUse(plainReq.event)) {
    const event = plainReq.event.protocolUse;
    const reqEvent = prehogApi.ConnectProtocolUseEvent.create({
      clusterName: event.clusterName,
      userName: event.userName,
      protocol: event.protocol,
      origin: event.origin,
    });

    req.event = {
      oneofKind: 'protocolUse',
      protocolUse: reqEvent,
    };

    return req;
  }
  if (connectEventOneOfIsAccessRequestCreate(plainReq.event)) {
    const event = plainReq.event.accessRequestCreate;
    const reqEvent = prehogApi.ConnectAccessRequestCreateEvent.create({
      clusterName: event.clusterName,
      userName: event.userName,
      kind: event.kind,
    });

    req.event = {
      oneofKind: 'accessRequestCreate',
      accessRequestCreate: reqEvent,
    };

    return req;
  }
  if (connectEventOneOfIsAccessRequestReview(plainReq.event)) {
    const event = plainReq.event.accessRequestReview;
    const reqEvent = prehogApi.ConnectAccessRequestReviewEvent.create({
      clusterName: event.clusterName,
      userName: event.userName,
    });

    req.event = {
      oneofKind: 'accessRequestReview',
      accessRequestReview: reqEvent,
    };

    return req;
  }
  if (connectEventOneOfIsAccessRequestAssumeRole(plainReq.event)) {
    const event = plainReq.event.accessRequestAssumeRole;
    const reqEvent = prehogApi.ConnectAccessRequestAssumeRoleEvent.create({
      clusterName: event.clusterName,
      userName: event.userName,
    });

    req.event = {
      oneofKind: 'accessRequestAssumeRole',
      accessRequestAssumeRole: reqEvent,
    };

    return req;
  }
  if (connectEventOneOfIsFileTransferRun(plainReq.event)) {
    const event = plainReq.event.fileTransferRun;
    const reqEvent = prehogApi.ConnectFileTransferRunEvent.create({
      clusterName: event.clusterName,
      userName: event.userName,
      isUpload: event.isUpload,
    });

    req.event = {
      oneofKind: 'fileTransferRun',
      fileTransferRun: reqEvent,
    };

    return req;
  }
  if (connectEventOneOfIsConnectMyComputerSetup(plainReq.event)) {
    const event = plainReq.event.connectMyComputerSetup;
    const reqEvent = prehogApi.ConnectConnectMyComputerSetup.create({
      clusterName: event.clusterName,
      userName: event.userName,
      success: event.success,
      failedStep: event.failedStep,
    });

    req.event = {
      oneofKind: 'connectMyComputerSetup',
      connectMyComputerSetup: reqEvent,
    };

    return req;
  }
  if (connectEventOneOfIsConnectMyComputerAgentStart(plainReq.event)) {
    const event = plainReq.event.connectMyComputerAgentStart;
    const reqEvent = prehogApi.ConnectConnectMyComputerAgentStart.create({
      clusterName: event.clusterName,
      userName: event.userName,
    });

    req.event = {
      oneofKind: 'connectMyComputerAgentStart',
      connectMyComputerAgentStart: reqEvent,
    };

    return req;
  }

  throw new Error(`Unrecognized event: ${JSON.stringify(plainReq)}`);
}
