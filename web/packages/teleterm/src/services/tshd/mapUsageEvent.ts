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

import { Timestamp } from 'google-protobuf/google/protobuf/timestamp_pb';
import * as api from 'gen-proto-js/teleport/lib/teleterm/v1/usage_events_pb';
import * as prehogApi from 'gen-proto-js/prehog/v1alpha/connect_pb';

import * as types from './types';

/**
 * Maps a plain JS object into a gRPC request object.
 */
export function mapUsageEvent(event: types.ReportUsageEventRequest) {
  return new api.ReportUsageEventRequest()
    .setAuthClusterId(event.authClusterId)
    .setPrehogReq(mapPrehogBody(event.prehogReq));
}

function mapPrehogBody(
  plainReq: types.ReportUsageEventRequest['prehogReq']
): prehogApi.SubmitConnectEventRequest {
  const req = new prehogApi.SubmitConnectEventRequest()
    .setTimestamp(Timestamp.fromDate(plainReq.timestamp))
    .setDistinctId(plainReq.distinctId);

  // Non-anonymized events.
  if (plainReq.userJobRoleUpdate) {
    const event = plainReq.userJobRoleUpdate;
    const reqEvent = new prehogApi.ConnectUserJobRoleUpdateEvent().setJobRole(
      event.jobRole
    );

    return req.setUserJobRoleUpdate(reqEvent);
  }

  // Anonymized events.
  if (plainReq.clusterLogin) {
    const event = plainReq.clusterLogin;
    const reqEvent = new prehogApi.ConnectClusterLoginEvent()
      .setClusterName(event.clusterName)
      .setUserName(event.userName)
      .setConnectorType(event.connectorType)
      .setOs(event.os)
      .setArch(event.arch)
      .setOsVersion(event.osVersion)
      .setAppVersion(event.appVersion);

    return req.setClusterLogin(reqEvent);
  }
  if (plainReq.protocolUse) {
    const event = plainReq.protocolUse;
    const reqEvent = new prehogApi.ConnectProtocolUseEvent()
      .setClusterName(event.clusterName)
      .setUserName(event.userName)
      .setProtocol(event.protocol)
      .setOrigin(event.origin);

    return req.setProtocolUse(reqEvent);
  }
  if (plainReq.accessRequestCreate) {
    const event = plainReq.accessRequestCreate;
    const reqEvent = new prehogApi.ConnectAccessRequestCreateEvent()
      .setClusterName(event.clusterName)
      .setUserName(event.userName)
      .setKind(event.kind);

    return req.setAccessRequestCreate(reqEvent);
  }
  if (plainReq.accessRequestReview) {
    const event = plainReq.accessRequestReview;
    const reqEvent = new prehogApi.ConnectAccessRequestReviewEvent()
      .setClusterName(event.clusterName)
      .setUserName(event.userName);

    return req.setAccessRequestReview(reqEvent);
  }
  if (plainReq.accessRequestAssumeRole) {
    const event = plainReq.accessRequestAssumeRole;
    const reqEvent = new prehogApi.ConnectAccessRequestAssumeRoleEvent()
      .setClusterName(event.clusterName)
      .setUserName(event.userName);

    return req.setAccessRequestAssumeRole(reqEvent);
  }
  if (plainReq.fileTransferRun) {
    const event = plainReq.fileTransferRun;
    const reqEvent = new prehogApi.ConnectFileTransferRunEvent()
      .setClusterName(event.clusterName)
      .setUserName(event.userName)
      .setIsUpload(event.isUpload);

    return req.setFileTransferRun(reqEvent);
  }
  if (plainReq.connectMyComputerSetup) {
    const event = plainReq.connectMyComputerSetup;
    const reqEvent = new prehogApi.ConnectConnectMyComputerSetup()
      .setClusterName(event.clusterName)
      .setUserName(event.userName)
      .setSuccess(event.success)
      .setFailedStep(event.failedStep);

    return req.setConnectMyComputerSetup(reqEvent);
  }
  if (plainReq.connectMyComputerAgentStart) {
    const event = plainReq.connectMyComputerAgentStart;
    const reqEvent = new prehogApi.ConnectConnectMyComputerAgentStart()
      .setClusterName(event.clusterName)
      .setUserName(event.userName);

    return req.setConnectMyComputerAgentStart(reqEvent);
  }

  throw new Error(`Unrecognized event: ${JSON.stringify(plainReq)}`);
}
