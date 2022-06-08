import api from 'teleport/services/api';
import sessionService, { RenewSessionRequest } from 'teleport/services/session';
import cfg from 'e-teleport/config';
import makeAccessRequest from './makeAccessRequest';
import {
  AccessRequestFilter,
  CreateAccessRequest,
  UpdateAccessRequest,
  AccessRequest,
} from './types';

class WorkflowService {
  fetchAccessRequest(requestId: string) {
    return api.get(cfg.getAccessRequestUrl(requestId)).then(makeAccessRequest);
  }

  fetchAccessRequests(filter: AccessRequestFilter): Promise<AccessRequest[]> {
    return api.get(cfg.getAccessRequestFilterUrl(filter)).then(requests => {
      if (!requests) {
        return [];
      }
      return requests.map(req => makeAccessRequest(req));
    });
  }

  createAccessRequest(request: CreateAccessRequest) {
    return api.post(cfg.getAccessRequestUrl(), request).then(makeAccessRequest);
  }

  submitAccessRequestReview(request: UpdateAccessRequest) {
    return api.put(cfg.getAccessRequestUrl(), request).then(makeAccessRequest);
  }

  deleteAccessRequest(requestId: string) {
    return api.delete(cfg.getAccessRequestUrl(requestId));
  }

  applyPermission(req: RenewSessionRequest) {
    return sessionService.renewSession(req);
  }
}

export default WorkflowService;
