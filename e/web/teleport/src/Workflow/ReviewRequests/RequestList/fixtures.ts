import {
  requestRoleApproved,
  requestRoleApprovedWithStartTime,
  requestRoleDenied,
  requestRolePending,
  requestRolePromoted,
  requestSearchPending,
} from 'shared/components/AccessRequests/fixtures';

import { State } from './useRequestList';

const requestRows = [
  {
    ...requestSearchPending,
    canAssume: false,
    isAssumed: false,
    ownRequest: false,
    isPromoted: false,
  },
  {
    ...requestRolePending,
    canAssume: false,
    isAssumed: false,
    ownRequest: false,
    isPromoted: false,
  },
  {
    ...requestRoleDenied,
    canAssume: false,
    isAssumed: false,
    ownRequest: false,
    isPromoted: false,
  },
  {
    ...requestRoleApproved,
    canAssume: true,
    isAssumed: false,
    ownRequest: false,
    isPromoted: false,
  },
  {
    ...requestRoleApproved,
    canAssume: true,
    isAssumed: true,
    ownRequest: false,
    isPromoted: false,
  },
  {
    ...requestRolePromoted,
    isPromoted: true,
    ownRequest: false,
    canAssume: false,
    isAssumed: false,
  },
  {
    ...requestRolePromoted,
    isPromoted: true,
    ownRequest: true,
    canAssume: false,
    isAssumed: false,
    requestReason: 'own promoted request',
  },
  {
    ...requestRoleApprovedWithStartTime,
    canAssume: true,
    isAssumed: false,
    ownRequest: false,
    isPromoted: false,
  },
];

export const sample: State = {
  attempt: {
    status: 'success' as any,
  },
  resources: requestRows,
  assumeRole: () => null,
  fetch: () => Promise.resolve(),
  updateSort: () => {},
  fetchAttempt: { status: '' },
  setSearchString: () => {},
  searchString: '',
  clear: () => {},
  updateScope: () => {},
  scope: '',
  sortBy: { fieldName: 'created', dir: 'ASC' },
};
