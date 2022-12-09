import api from 'teleport/services/api';

import Workflow from './workflow';

test('handling of empty access request list', async () => {
  // Test null response.
  jest.spyOn(api, 'get').mockResolvedValue(null);

  const workflow = new Workflow();
  const response = await workflow.fetchAccessRequests({});

  expect(response).not.toBeNull();
  expect(response).toHaveLength(0);
});

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
  jest.spyOn(api, 'get').mockResolvedValue(requestApproved);

  const workflow = new Workflow();
  const response = await workflow.fetchAccessRequest('123');

  expect(response).toEqual({
    id: '72de9b90-04fd-5621-a55d-432d9fe56ef2',
    state: 'APPROVED',
    user: 'Sam',
    expires: undefined,
    expiresDuration: '',
    created: undefined,
    createdDuration: '',
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
      },
      {
        author: 'alice',
        state: 'DENIED',
        reason: '',
        roles: ['admin'],
        createdDuration: '',
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
};
