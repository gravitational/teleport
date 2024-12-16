/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { AccessRequest } from 'shared/services/accessRequests';

export const dryRunResponse: AccessRequest = {
  id: 'e9803adc-3260-4c49-baae-047494da2822',
  state: 'PENDING',
  resolveReason: '',
  requestReason: '',
  user: 'lisa',
  roles: ['auditor'],
  created: new Date('2024-02-15T02:51:00.000088Z'),
  createdDuration: '',
  expires: new Date('2024-02-17T02:51:12.70087Z'),
  expiresDuration: '',
  maxDuration: new Date('2024-02-17T02:51:12.70087Z'),
  maxDurationText: '',
  requestTTL: new Date('2024-02-15T03:51:12.70087Z'),
  requestTTLDuration: '',
  sessionTTL: new Date('2024-02-15T14:51:03.999893Z'),
  sessionTTLDuration: '',
  reviews: [],
  reviewers: [
    { name: 'bob', state: '' },
    { name: 'cat', state: '' },
    { name: 'george washington', state: '' },
  ],
  thresholdNames: ['default'],
  resources: [],
  assumeStartTime: null,
};

export const requestSearchPending: AccessRequest = {
  id: '461ff4bb-62f1-53b5-84ae-731022261a12',
  state: 'PENDING',
  user: 'Sam',
  expires: new Date('12-6-2020'),
  expiresDuration: '35 minutes',
  created: new Date('12-4-2020'),
  createdDuration: '1 minute ago',
  maxDuration: new Date('12-6-2020'),
  maxDurationText: '',
  requestTTL: new Date('12-5-2020'),
  requestTTLDuration: '1 hour',
  sessionTTL: new Date('12-5-2020'),
  sessionTTLDuration: '',
  roles: ['test'],
  requestReason:
    'Testing long message format. I am requesting access for the developer role that i will be using to \
        commit fixes for our production application. I will need access for the \
        rest of the day to complete my changes.',
  resolveReason: '',
  reviews: [],
  reviewers: [
    {
      name: 'alice',
      state: 'PENDING',
    },
    {
      name: 'bob',
      state: 'PENDING',
    },
  ],
  thresholdNames: ['Default', 'Poplar', 'Admin'],
  resources: [
    {
      id: {
        kind: 'app',
        name: 'app-name',
        clusterName: 'cluster-name',
      },
    },
    {
      id: {
        kind: 'db',
        name: 'db-name',
        clusterName: 'cluster-name',
      },
    },
    {
      id: {
        kind: 'node',
        name: 'node-name',
        clusterName: 'cluster-name',
      },
    },
    {
      id: {
        kind: 'user_group',
        name: 'user-group-name',
        clusterName: 'cluster-name',
      },
    },
    {
      id: {
        kind: 'kube_cluster',
        name: 'kube-cluster-name',
        clusterName: 'cluster-name',
      },
    },
    {
      id: {
        kind: 'windows_desktop',
        name: 'windows-desktop-name',
        clusterName: 'cluster-name',
      },
    },
    {
      id: {
        kind: 'app',
        name: 'raw-id',
        clusterName: 'cluster-name',
      },
      details: {
        friendlyName: 'Some Friendly Name',
      },
    },
    {
      id: {
        kind: 'saml_idp_service_provider',
        name: 'raw-saml-id',
        clusterName: 'cluster-name',
      },
      details: {
        friendlyName: 'app-saml',
      },
    },
    {
      id: {
        kind: 'aws_ic_account_assignment',
        name: 'admin-on-account1',
        clusterName: 'cluster-name',
      },
      details: {
        friendlyName: 'account1',
      },
    },
  ],
};

export const requestRolePending: AccessRequest = {
  id: '461ff4bb-62f1-53b5-84ae-731022261a12',
  state: 'PENDING',
  user: 'Sam',
  expires: new Date('12-6-2020'),
  expiresDuration: '35 minutes',
  created: new Date('12-4-2020'),
  createdDuration: '1 minute ago',
  maxDuration: new Date('12-6-2020'),
  maxDurationText: '',
  requestTTL: new Date('12-6-2020'),
  requestTTLDuration: '2 hours',
  sessionTTL: new Date('12-6-2020'),
  sessionTTLDuration: '',
  roles: ['admin'],
  requestReason:
    'Testing long message format. I am requesting access for the developer role that i will be using to \
        commit fixes for our production application. I will need access for the \
        rest of the day to complete my changes.',
  resolveReason: '',
  reviews: [],
  reviewers: [
    {
      name: 'alice',
      state: 'PENDING',
    },
    {
      name: 'bob',
      state: 'PENDING',
    },
  ],
  thresholdNames: ['Default', 'Poplar', 'Admin'],
  resources: [],
  assumeStartTime: new Date('12-5-2020'),
  assumeStartTimeDuration: '24 hours from now',
};

export const requestRoleDenied: AccessRequest = {
  id: '3ce23da9-6b85-5fce-9bf3-5fb826120cb2',
  state: 'DENIED',
  user: 'Sam',
  expires: new Date('12-6-2020'),
  expiresDuration: '20 hours',
  created: new Date('12-2-2020'),
  createdDuration: '35 minutes ago',
  maxDuration: new Date('12-6-2020'),
  maxDurationText: '',
  requestTTL: new Date('12-5-2020'),
  requestTTLDuration: '1 hour',
  sessionTTL: new Date('12-5-2020'),
  sessionTTLDuration: '',
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
    {
      name: 'alice',
      state: 'DENIED',
    },
    {
      name: 'bob',
      state: 'PENDING',
    },
  ],
  thresholdNames: ['Default'],
  resources: [],
};

export const requestRoleApproved: AccessRequest = {
  id: '72de9b90-04fd-5621-a55d-432d9fe56ef2',
  state: 'APPROVED',
  user: 'Sam',
  expires: new Date('12-6-2020'),
  expiresDuration: '24 hours',
  created: new Date('12-1-2020'),
  createdDuration: '2 hours ago',
  maxDuration: new Date('12-6-2020'),
  maxDurationText: '',
  requestTTL: new Date('12-5-2020'),
  requestTTLDuration: '2 hours',
  sessionTTL: new Date('12-5-2020'),
  sessionTTLDuration: '',
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
    {
      name: 'alice',
      state: 'APPROVED',
    },
    {
      name: 'bob',
      state: 'PENDING',
    },
    {
      name: 'test-long-user-name@testing.com',
      state: 'APPROVED',
    },
  ],
  thresholdNames: ['Default'],
  resources: [],
};

export const requestRoleApprovedWithStartTime: AccessRequest = {
  id: '72de9b90-04fd-5621-a55d-432d9fe56ef2',
  state: 'APPROVED',
  user: 'Sam',
  expires: new Date('12-6-2020'),
  expiresDuration: '24 hours',
  created: new Date('12-1-2020'),
  createdDuration: '2 hours ago',
  maxDuration: new Date('12-6-2020'),
  maxDurationText: '',
  requestTTL: new Date('12-5-2020'),
  requestTTLDuration: '2 hours',
  sessionTTL: new Date('12-5-2020'),
  sessionTTLDuration: '',
  roles: ['kaco', 'ziuzzow', 'admin'],
  requestReason: '',
  resolveReason: '',
  reviews: [
    {
      author: 'test-long-user-name@testing.com',
      createdDuration: '1 minute ago',
      reason: '',
      state: 'APPROVED',
      roles: ['admin'],
    },
  ],
  reviewers: [
    {
      name: 'alice',
      state: 'APPROVED',
    },
  ],
  thresholdNames: ['Default'],
  resources: [],
  assumeStartTime: new Date('12-6-9999'),
  assumeStartTimeDuration: '24 hours from now',
};

export const requestRolePromoted: AccessRequest = {
  id: '72de9b90-04fd-5621-a55d-432d9fe56ef2',
  state: 'PROMOTED',
  user: 'Sam',
  expires: new Date('12-6-2020'),
  expiresDuration: '24 hours',
  created: new Date('12-1-2020'),
  createdDuration: '2 hours ago',
  maxDuration: new Date('12-6-2020'),
  maxDurationText: '24 hours',
  requestTTL: new Date('12-5-2020'),
  requestTTLDuration: '',
  sessionTTL: new Date('12-5-2020'),
  sessionTTLDuration: '',
  roles: ['kaco', 'ziuzzow', 'admin'],
  requestReason: '',
  resolveReason: '',
  reviews: [
    {
      author: 'george.washington.first.president@testing.com',
      createdDuration: '1 minute ago',
      reason: '',
      promotedAccessListTitle: 'Design Team',
      roles: ['admin'],
      state: 'PROMOTED',
    },
  ],
  reviewers: [
    {
      name: 'george.washington.first.president@testing.com',
      state: 'PROMOTED',
    },
  ],
  thresholdNames: ['Default'],
  resources: [],
  promotedAccessListTitle: 'Design Team',
};

export const requestRoleEmpty: AccessRequest = {
  ...requestRoleApproved,
  reviews: [],
  reviewers: [],
  roles: ['empty-values'],
  id: 'ffc11a95-e8af-581c-ba82-47c429c841e8',
};

export const requests = [
  requestRolePending,
  requestRoleDenied,
  requestRoleApproved,
];
