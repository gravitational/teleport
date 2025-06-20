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

import { AccessRequest as TshdAccessRequest } from 'gen-proto-ts/teleport/lib/teleterm/v1/access_request_pb';
import { AccessRequest, RequestKind } from 'shared/services/accessRequests';

import { makeUiAccessRequest } from './useAccessRequests';

test('makeUiAccessRequest', async () => {
  jest.useFakeTimers();
  jest.setSystemTime(new Date('2024-03-12T00:00:00.000Z'));

  const request: TshdAccessRequest = {
    id: '018e1246-0f5c-7829-8b05-8efac30fe18e',
    state: 'PENDING',
    resolveReason: 'resolve-reason',
    requestReason: 'request-reason',
    user: 'sevy',
    roles: ['access'],
    reviews: [
      {
        author: 'llama',
        state: 'DENIED',
        roles: ['admin'],
        reason: 'not today',
        promotedAccessListTitle: '',
        created: { seconds: 1709703565n, nanos: 148537000 },
      },
    ],
    suggestedReviewers: ['sugested-reviewer-1'],
    thresholdNames: ['default'],
    resourceIds: [],
    resources: [
      {
        id: {
          name: 'name',
          kind: 'node',
          clusterName: 'cluster',
          subResourceName: 'subResourceName',
        },
      },
      {
        id: {
          clusterName: 'cluster',
          name: 'name',
          kind: 'node',
          subResourceName: 'subResourceName',
        },
        details: { hostname: 'hostname', friendlyName: 'friendlyName' },
      },
    ],
    promotedAccessListTitle: 'promoted-title',
    created: {
      seconds: 1709703565n,
      nanos: 148537000,
    },
    expires: {
      seconds: 1709746587n,
      nanos: 999998000,
    },
    maxDuration: {
      seconds: 1709746587n,
      nanos: 999998000,
    },
    requestTtl: {
      seconds: 1710308365n,
      nanos: 148880000,
    },
    sessionTtl: {
      seconds: 1709746587n,
      nanos: 999998000,
    },
    assumeStartTime: {
      seconds: 1709853650n,
      nanos: 520000000,
    },
    reasonMode: 'optional',
    reasonPrompts: [],
  };

  const processedRequest: AccessRequest = {
    created: new Date('2024-03-06T05:39:25.149Z'),
    createdDuration: '6 days ago',
    expires: new Date('2024-03-06T17:36:28.000Z'),
    expiresDuration: '5 days',
    id: '018e1246-0f5c-7829-8b05-8efac30fe18e',
    maxDuration: new Date('2024-03-06T17:36:28.000Z'),
    maxDurationText: '5 days',
    promotedAccessListTitle: 'promoted-title',
    requestReason: 'request-reason',
    requestTTL: new Date('2024-03-13T05:39:25.149Z'),
    requestTTLDuration: '1 day',
    resolveReason: 'resolve-reason',
    resources: [
      {
        id: {
          clusterName: 'cluster',
          kind: 'node',
          name: 'name',
          subResourceName: 'subResourceName',
        },
      },
      {
        details: {
          friendlyName: 'friendlyName',
          hostname: 'hostname',
        },
        id: {
          clusterName: 'cluster',
          kind: 'node',
          name: 'name',
          subResourceName: 'subResourceName',
        },
      },
    ],
    reviewers: [
      {
        name: 'sugested-reviewer-1',
        state: 'PENDING',
      },
      {
        name: 'llama',
        state: 'DENIED',
      },
    ],
    reviews: [
      {
        author: 'llama',
        createdDuration: '6 days ago',
        promotedAccessListTitle: '',
        reason: 'not today',
        roles: ['admin'],
        state: 'DENIED',
        assumeStartTime: null,
      },
    ],
    roles: ['access'],
    sessionTTL: new Date('2024-03-06T17:36:28.000Z'),
    sessionTTLDuration: '5 days',
    state: 'PENDING',
    thresholdNames: ['default'],
    user: 'sevy',
    assumeStartTime: new Date('2024-03-07T23:20:50.520Z'),
    assumeStartTimeDuration: 'now',
    reasonMode: 'optional',
    reasonPrompts: [],
    requestKind: RequestKind.UNDEFINED,
    longTermResourceGrouping: undefined,
  };

  expect(makeUiAccessRequest(request)).toStrictEqual(processedRequest);
});
