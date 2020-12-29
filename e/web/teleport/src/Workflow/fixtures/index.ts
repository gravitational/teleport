import { CreateAccessRequest } from 'e-teleport/services/workflow';
import StoreAccessRequests from 'e-teleport/stores/storeAccessRequests';

export const requestPending = {
  id: '461ff4bb-62f1-53b5-84ae-731022261a12',
  state: 'PENDING' as any,
  user: 'Sam',
  expires: new Date(0),
  expiresDuration: '35 minutes',
  created: new Date('12-4-2020'),
  createdDuration: '1 minute ago',
  roles: ['admin'],
  requestReason:
    'Testing long message format. I am requesting access for the developer role that i will be using to \
    commit fixes for our production application. I will need access for the \
    rest of the day to complete my changes.',
  resolveReason: '',
};

export const requestDenied = {
  id: '3ce23da9-6b85-5fce-9bf3-5fb826120cb2',
  state: 'DENIED' as any,
  user: 'Sam',
  expires: new Date(0),
  expiresDuration: '20 hours',
  created: new Date('12-2-2020'),
  createdDuration: '35 minutes ago',
  roles: ['ruhh', 'admin'],
  requestReason: '',
  resolveReason: '',
};

export const requestApproved = {
  id: '72de9b90-04fd-5621-a55d-432d9fe56ef2',
  state: 'APPROVED' as any,
  user: 'Sam',
  expires: new Date(0),
  expiresDuration: '24 hours',
  created: new Date('12-1-2020'),
  createdDuration: '2 hours ago',
  roles: ['kaco', 'ziuzzow', 'admin'],
  requestReason: '',
  resolveReason: '',
};

const assumedRequest = {
  ...requestApproved,
  id: 'assumed@button',
  requestReason: 'show assumed button state',
};

export const requests = [
  requestPending,
  requestDenied,
  requestApproved,
  assumedRequest,
];

export class MockedWorkflowService {
  requests = [];
  user = '';

  constructor(ctxUser) {
    this.requests = JSON.parse(JSON.stringify(requests));
    this.user = ctxUser;
  }

  fetchAccessRequests = () => Promise.resolve(this.requests);

  fetchAccessRequest = (requestId: string) => {
    return Promise.resolve(this.requests.find(r => r.id === requestId));
  };

  createAccessRequest = (req: CreateAccessRequest) => {
    const request = {
      ...requestPending,
      user: this.user,
      createdDuration: 'a few seconds ago',
      roles: req.roles,
      requestReason: req.reason,
    };
    this.requests.push(request);
    return Promise.resolve(request);
  };

  applyPermission = () => Promise.resolve();
}

export class MockedStoreAccessRequests extends StoreAccessRequests {
  getAssumedRequests = () => ({
    [assumedRequest.id]: assumedRequest,
  });

  isAssumed = (requestId: string) => requestId === assumedRequest.id;

  // Disable local storage settings.
  setState = () => {};
  setApprovedWaitingRoomRequest = () => {};
}
