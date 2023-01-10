import { Timestamp } from 'google-protobuf/google/protobuf/timestamp_pb';

import * as api from './v1/usage_events_pb';
import * as prehogApi from './prehog/v1alpha/connect_pb';

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
      .setProtocol(event.protocol);

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

  throw new Error(`Unrecognized event: ${JSON.stringify(plainReq)}`);
}
