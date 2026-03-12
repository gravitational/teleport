import { makeAccessRequest, ResourceId } from 'shared/services/accessRequests';

import cfg from 'e-teleport/config';
import { ResourcesResponse } from 'teleport/services/agents';
import api from 'teleport/services/api';
import session, { RenewSessionRequest } from 'teleport/services/websession';

import {
  AccessRequest,
  AccessRequestFilter,
  CreateAccessRequest,
  PromoteAccessRequest,
  UpdateAccessRequest,
} from './types';

class WorkflowService {
  fetchAccessRequest(requestId: string) {
    return api.get(cfg.getAccessRequestUrl(requestId)).then(makeAccessRequest);
  }

  fetchAccessRequests(
    filter: AccessRequestFilter,
    signal: AbortSignal
  ): Promise<ResourcesResponse<AccessRequest>> {
    return api.get(cfg.getAccessRequestFilterUrl(filter), signal);
  }

  fetchResourceRequestRoles(
    resourceIds: ResourceId[],
    signal?: AbortSignal
  ): Promise<string[]> {
    return api
      .get(cfg.getResourceRequestRolesUrl(resourceIds), signal)
      .then(roles => roles || []);
  }

  createAccessRequest(
    request: CreateAccessRequest,
    signal: AbortSignal = null
  ) {
    return api
      .post(cfg.getAccessRequestUrl(), request, signal)
      .then(makeAccessRequest);
  }

  submitAccessRequestReview(request: UpdateAccessRequest) {
    return api.put(cfg.getAccessRequestUrl(), request).then(makeAccessRequest);
  }

  promoteAccessRequest(requestId: string, request: PromoteAccessRequest) {
    return api
      .post(cfg.getAccessRequestPromoteUrl(requestId), request)
      .then(res => makeAccessRequest(res?.accessRequest));
  }

  deleteAccessRequest(requestId: string) {
    return api.delete(cfg.getAccessRequestUrl(requestId));
  }

  applyPermission(req: RenewSessionRequest) {
    return session.renewSession(req);
  }
}

export default WorkflowService;
