import StoreAccessRequests from 'e-teleport/stores/storeAccessRequests';
import { AccessRequest } from 'e-teleport/services/workflow';

export const requestPending: AccessRequest = {
  id: '461ff4bb-62f1-53b5-84ae-731022261a12',
  state: 'PENDING',
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
  reviews: [],
  reviewers: [
    { name: 'alice', state: 'PENDING' },
    { name: 'bob', state: 'PENDING' },
  ],
  thresholdNames: ['Default', 'Poplar', 'Admin'],
};

export const requestDenied: AccessRequest = {
  id: '3ce23da9-6b85-5fce-9bf3-5fb826120cb2',
  state: 'DENIED',
  user: 'Sam',
  expires: new Date(0),
  expiresDuration: '20 hours',
  created: new Date('12-2-2020'),
  createdDuration: '35 minutes ago',
  roles: ['ruhh', 'admin'],
  requestReason: 'Some short request reason',
  resolveReason: '',
  reviews: [
    {
      author: 'alice',
      createdDuration: '26 hours ago',
      state: 'DENIED',
      reason: 'Not today',
      roles: ['admin', 'developer'],
    },
  ],
  reviewers: [
    { name: 'alice', state: 'DENIED' },
    { name: 'bob', state: 'PENDING' },
  ],
  thresholdNames: ['Default'],
};

export const requestApproved: AccessRequest = {
  id: '72de9b90-04fd-5621-a55d-432d9fe56ef2',
  state: 'APPROVED',
  user: 'Sam',
  expires: new Date(0),
  expiresDuration: '24 hours',
  created: new Date('12-1-2020'),
  createdDuration: '2 hours ago',
  roles: ['kaco', 'ziuzzow', 'admin'],
  requestReason: '',
  resolveReason: '',
  reviews: [
    {
      author: 'alice',
      createdDuration: '26 hours ago',
      reason:
        'Approving for developer role not admin. Admins access is not needed for this request.',
      state: 'APPROVED',
      roles: ['kaco', 'admin'],
    },
    {
      author: 'test-long-user-name@testing.com',
      createdDuration: '1 minute ago',
      reason: '',
      state: 'APPROVED',
      roles: ['admin'],
    },
  ],
  reviewers: [
    { name: 'alice', state: 'APPROVED' },
    { name: 'bob', state: 'PENDING' },
    { name: 'test-long-user-name@testing.com', state: 'APPROVED' },
  ],
  thresholdNames: ['Default'],
};

export const requestEmpty: AccessRequest = {
  ...requestApproved,
  reviews: [],
  reviewers: [],
  roles: ['empty-values'],
  id: 'ffc11a95-e8af-581c-ba82-47c429c841e8',
};

export const requests = [requestPending, requestDenied, requestApproved];

export class MockedWorkflowService {
  requests = [];
  user = '';

  constructor() {
    this.requests = JSON.parse(JSON.stringify(requests));
  }

  fetchAccessRequests = () => {
    return Promise.resolve(this.requests);
  };

  fetchAccessRequest = () => {
    return Promise.resolve(requestPending);
  };

  createAccessRequest = () => {
    return Promise.reject(new Error('not implemented'));
  };

  applyPermission = () => Promise.resolve(null);
  submitAccessRequestReview = () => Promise.resolve(requestApproved);
  deleteAccessRequest = () => Promise.resolve();
}

export class MockedStoreAccessRequests extends StoreAccessRequests {
  getAssumedRequests = () => ({
    [requestApproved.id]: requestApproved,
  });

  isAssumed = () => false;

  // Disable local storage settings.
  setState = () => {};
  setApprovedWaitingRoomRequest = () => {};
}
