/**
 * Copyright 2024 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
