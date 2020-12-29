import api from 'teleport/services/api';
import sessionService from 'teleport/services/session';
import cfg from 'e-teleport/config';
import makeAccessRequest from './makeAccessRequest';
import { AccessRequestFilter, CreateAccessRequest } from './types';

class WorkflowService {
  fetchAccessRequest(requestId: string) {
    return api.get(cfg.getAccessRequestUrl(requestId)).then(makeAccessRequest);
  }

  fetchAccessRequests(filter: AccessRequestFilter) {
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

  applyPermission(requestId: string) {
    return sessionService.renewSession(requestId);
  }
}

export default WorkflowService;
