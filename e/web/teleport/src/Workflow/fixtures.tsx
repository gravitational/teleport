import {
  requestRoleApproved,
  requestRolePending,
  requestRolePromoted,
  requests,
} from 'shared/components/AccessRequests/fixtures';

import { AccessRequest } from 'e-teleport/services/workflow';
import StoreAccessRequests from 'e-teleport/stores/storeAccessRequests';
import { ResourcesResponse } from 'teleport/services/agents';

export class MockedWorkflowService {
  requests: ResourcesResponse<AccessRequest> = { agents: [] };
  user = '';

  constructor() {
    this.requests = JSON.parse(JSON.stringify(requests));
  }

  fetchAccessRequests = () => {
    return Promise.resolve(this.requests);
  };

  fetchAccessRequest = () => {
    return Promise.resolve(requestRolePending);
  };

  promoteAccessRequest = () => {
    return Promise.resolve(requestRolePromoted);
  };

  fetchResourceRequestRoles = () => {
    return Promise.resolve(requestRoleApproved.roles);
  };

  createAccessRequest = () => {
    return Promise.reject(new Error('not implemented'));
  };

  applyPermission = () => Promise.resolve(null);
  submitAccessRequestReview = () => Promise.resolve(requestRoleApproved);
  deleteAccessRequest = () => Promise.resolve();
}

export class MockedStoreAccessRequests extends StoreAccessRequests {
  getAssumedRequests = () => ({
    [requestRoleApproved.id]: requestRoleApproved,
  });

  isAssumed = () => false;

  // Disable local storage settings.
  setState = () => {};
  setApprovedWaitingRoomRequest = () => {};
}
