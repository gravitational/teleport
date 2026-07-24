/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { makeAccessRequest } from './makeAccessRequest';

test('maps current user info fields and display values', () => {
  const request = makeAccessRequest({
    ...requestJson,
    user: 'legacy-requester',
    userInfo: {
      username: 'requester',
      display: {
        primary: 'Requesting User',
        secondary: 'requester@example.com',
      },
    },
    suggestedReviewers: ['legacy-reviewer'],
    suggestedReviewersInfo: [
      {
        username: 'reviewer-one',
        display: { primary: 'Shared Reviewer' },
      },
      {
        username: 'reviewer-two',
        display: { primary: 'Shared Reviewer' },
      },
      {
        username: 'reviewer-empty',
        display: {},
      },
      {
        username: 'reviewer-secondary',
        display: { secondary: 'secondary@example.com' },
      },
    ],
    reviews: [
      {
        author: 'legacy-author',
        authorInfo: {
          username: 'reviewer-one',
          display: { primary: 'Shared Reviewer' },
        },
        state: 'APPROVED',
        reason: 'Approved',
        roles: ['editor'],
        created: '2026-07-16T12:30:00.000Z',
      },
      {
        author: 'legacy-unsuggested-author',
        authorInfo: {
          username: 'unsuggested-reviewer',
          display: { primary: 'Unlisted Reviewer' },
        },
        state: 'DENIED',
        reason: 'Denied',
        roles: [],
        created: '2026-07-16T12:45:00.000Z',
      },
    ],
  });

  expect(request).toMatchObject({
    user: 'requester',
    userDisplay: {
      primary: 'Requesting User',
      secondary: 'requester@example.com',
    },
    reviews: [
      {
        author: 'reviewer-one',
        authorDisplay: { primary: 'Shared Reviewer' },
      },
      {
        author: 'unsuggested-reviewer',
        authorDisplay: { primary: 'Unlisted Reviewer' },
      },
    ],
    reviewers: [
      {
        name: 'reviewer-one',
        display: { primary: 'Shared Reviewer' },
        state: 'APPROVED',
      },
      {
        name: 'reviewer-two',
        display: { primary: 'Shared Reviewer' },
        state: 'PENDING',
      },
      {
        name: 'reviewer-empty',
        display: {},
        state: 'PENDING',
      },
      {
        name: 'reviewer-secondary',
        display: { secondary: 'secondary@example.com' },
        state: 'PENDING',
      },
      {
        name: 'unsuggested-reviewer',
        display: { primary: 'Unlisted Reviewer' },
        state: 'DENIED',
      },
    ],
  });
});

test('falls back to legacy usernames when current info fields are absent', () => {
  const request = makeAccessRequest({
    ...requestJson,
    user: 'legacy-requester',
    suggestedReviewers: ['legacy-reviewer'],
    reviews: [
      {
        author: 'legacy-author',
        state: 'APPROVED',
        reason: '',
        roles: [],
        created: '2026-07-16T12:30:00.000Z',
      },
    ],
  });

  expect(request).toMatchObject({
    user: 'legacy-requester',
    userDisplay: undefined,
    reviews: [
      {
        author: 'legacy-author',
        authorDisplay: undefined,
      },
    ],
    reviewers: [
      {
        name: 'legacy-reviewer',
        display: undefined,
        state: 'PENDING',
      },
      {
        name: 'legacy-author',
        display: undefined,
        state: 'APPROVED',
      },
    ],
  });
  expect(request).toHaveProperty('userDisplay', undefined);
  expect(request.reviews[0]).toHaveProperty('authorDisplay', undefined);
  expect(request.reviewers[0]).toHaveProperty('display', undefined);
  expect(request.reviewers[1]).toHaveProperty('display', undefined);
});

test('preserves the difference between empty and absent displays', () => {
  const request = makeAccessRequest({
    ...requestJson,
    userInfo: { username: 'requester', display: {} },
    suggestedReviewersInfo: [
      { username: 'empty-display', display: {} },
      { username: 'absent-display' },
    ],
  });

  expect(request.user).toBe('requester');
  expect(request).toHaveProperty('userDisplay', {});
  expect(request.reviewers[0]).toHaveProperty('display', {});
  expect(request.reviewers[1]).toHaveProperty('display', undefined);
});

const requestJson = {
  id: 'request-id',
  state: 'PENDING',
  expires: '2026-07-17T12:00:00.000Z',
  created: '2026-07-16T12:00:00.000Z',
  requestTTL: '2026-07-17T12:00:00.000Z',
  roles: ['editor'],
};
