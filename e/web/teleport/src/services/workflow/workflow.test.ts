import api from 'teleport/services/api';

import Workflow from './workflow';

test('handling of empty lists in an access request', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(null);

  const workflow = new Workflow();
  const response = await workflow.fetchAccessRequest('123');

  expect(response.roles).toHaveLength(0);
  expect(response.thresholdNames).toHaveLength(0);
  expect(response.reviews).toHaveLength(0);
  expect(response.reviewers).toHaveLength(0);
  expect(response.resources).toHaveLength(0);
});

test('handling of empty resource request roles response', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(null);

  const workflow = new Workflow();
  const response = await workflow.fetchResourceRequestRoles([
    { clusterName: 'cluster', name: 'name', kind: 'app' },
  ]);

  expect(response).toHaveLength(0);
});

test('correct formatting of access request json response', async () => {
  jest.useFakeTimers().setSystemTime(new Date('2024-02-15'));

  jest.spyOn(api, 'get').mockResolvedValue(requestApproved);

  const workflow = new Workflow();
  const response = await workflow.fetchAccessRequest('123');

  expect(response).toEqual({
    assumeStartTime: null,
    assumeStartTimeDuration: 'now',
    id: '72de9b90-04fd-5621-a55d-432d9fe56ef2',
    longTermResourceGrouping: undefined,
    state: 'APPROVED',
    user: 'Sam',
    expires: new Date('2024-02-15T11:56:43.482795Z'),
    expiresDuration: '12 hours',
    created: new Date('2024-02-15T03:56:45.533492Z'),
    createdDuration: 'in 4 hours',
    maxDuration: new Date('2024-02-15T11:56:43.482795Z'),
    maxDurationText: '12 hours',
    promotedAccessListTitle: undefined,
    requestTTL: new Date('2024-02-15T11:56:43.480999Z'),
    requestTTLDuration: '12 hours',
    sessionTTL: new Date('2024-02-15T11:56:43.482795Z'),
    sessionTTLDuration: '12 hours',
    roles: ['dev', 'admin'],
    resolveReason: 'resolve reason',
    requestReason: 'request reason',
    reviews: [
      {
        author: 'may',
        state: 'APPROVED',
        reason: 'some reason',
        roles: ['admin'],
        createdDuration: '',
        assumeStartTime: null,
        promotedAccessListTitle: undefined,
      },
      {
        author: 'alice',
        state: 'DENIED',
        reason: '',
        roles: ['admin'],
        createdDuration: '',
        assumeStartTime: null,
        promotedAccessListTitle: undefined,
      },
    ],
    // Reviewers should contain both review authors and suggested reviewers.
    // Suggested reviewers who did not review should remain pending.
    reviewers: [
      { name: 'alice', state: 'DENIED' },
      { name: 'bob', state: 'PENDING' },
      { name: 'may', state: 'APPROVED' },
    ],
    thresholdNames: ['Default'],
    resources: [
      { id: { clusterName: 'cluster', Name: 'name', Kind: 'kind' } },
      {
        id: { clusterName: 'cluster', Name: 'name', Kind: 'kind' },
        details: { hostname: 'hostname' },
      },
    ],
    reasonMode: 'optional',
    reasonPrompts: [],
    requestKind: 0,
  });
});

test('correct formatting of assume start time', async () => {
  jest.useFakeTimers().setSystemTime(new Date('2020-01-18'));

  jest.spyOn(api, 'get').mockResolvedValue({
    ...requestApproved,
    assumeStartTime: new Date('2020-01-19'),
  });

  const workflow = new Workflow();
  const response = await workflow.fetchAccessRequest('123');

  expect(response).toMatchObject({
    assumeStartTime: new Date('2020-01-19'),
    assumeStartTimeDuration: '1 day from now',
  });
});

const requestApproved = {
  id: '72de9b90-04fd-5621-a55d-432d9fe56ef2',
  state: 'APPROVED',
  user: 'Sam',
  roles: ['dev', 'admin'],
  requestReason: 'request reason',
  resolveReason: 'resolve reason',
  reviews: [
    {
      author: 'may',
      reason: 'some reason',
      state: 'APPROVED',
      roles: ['admin'],
    },
    {
      author: 'alice',
      reason: '',
      state: 'DENIED',
      roles: ['admin'],
    },
  ],
  suggestedReviewers: ['alice', 'bob'],
  thresholdNames: ['Default'],
  resources: [
    { id: { clusterName: 'cluster', Name: 'name', Kind: 'kind' } },
    {
      id: { clusterName: 'cluster', Name: 'name', Kind: 'kind' },
      details: { hostname: 'hostname' },
    },
  ],
  expires: new Date('2024-02-15T11:56:43.482795Z'),
  created: new Date('2024-02-15T03:56:45.533492Z'),
  maxDuration: new Date('2024-02-15T11:56:43.482795Z'),
  requestTTL: new Date('2024-02-15T11:56:43.480999Z'),
  sessionTTL: new Date('2024-02-15T11:56:43.482795Z'),
};
