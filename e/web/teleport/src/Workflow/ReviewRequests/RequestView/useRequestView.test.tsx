import React from 'react';
import { MemoryRouter, Route } from 'react-router';
import { waitFor } from '@testing-library/react';
import renderHook, { act } from 'design/utils/renderHook';
import makeUserContext from 'teleport/services/user/makeUserContext';

import TeleportContextE from 'e-teleport/teleportContextE';
import { requestRolePending } from 'e-teleport/Workflow/fixtures';
import { accessManagementService } from 'e-teleport/services/accessmanagement';

import useRequestView from './useRequestView';

test('flags for own request', async () => {
  const ctx = new TeleportContextE();
  ctx.storeUser.setState(userContext);
  ctx.storeAccessRequests.isAssumed = () => false;
  ctx.workflowService.fetchAccessRequest = () =>
    Promise.resolve(requestRolePending);
  accessManagementService.fetchAccessListSuggestions = () =>
    Promise.resolve([]);
  ctx.workflowService.submitAccessRequestReview = () =>
    Promise.resolve({ ...requestRolePending, state: 'APPROVED' });

  const utils = renderHook(() => useRequestView(ctx), {
    wrapper: Wrapper,
  });

  // test on mount, request and flags are init

  await waitFor(() => {
    expect(utils.current.request.state).toBe('PENDING');
  });
  expect(utils.current.user).toBe('Sam');

  expect(utils.current.request.id).toEqual(requestRolePending.id);
  expect(utils.current.flags).toEqual({
    canAssume: false,
    isAssumed: false,
    canDelete: true,
    canReview: false,
    ownRequest: true,
    isPromoted: false,
  });

  // test setting of request and flags after request is approved
  await act(() =>
    utils.current.submitReview({ state: 'APPROVED', reason: '' })
  );
  await waitFor(() => {
    expect(utils.current.request.state).toBe('APPROVED');
  });
  expect(utils.current.flags).toEqual({
    canAssume: true,
    isAssumed: false,
    canDelete: true,
    canReview: false,
    ownRequest: true,
    isPromoted: false,
  });

  // test review isAssumed flag
  ctx.storeAccessRequests.isAssumed = () => true;
  await act(() =>
    utils.current.submitReview({ state: 'APPROVED', reason: '' })
  );

  await waitFor(() => {
    expect(utils.current.flags).toEqual({
      canAssume: true,
      isAssumed: true,
      canDelete: true,
      canReview: false,
      ownRequest: true,
      isPromoted: false,
    });
  });

  // test setting of request and flags after request is denied
  ctx.storeAccessRequests.isAssumed = () => false;
  ctx.workflowService.submitAccessRequestReview = () =>
    Promise.resolve({ ...requestRolePending, state: 'DENIED' });
  await act(() => utils.current.submitReview({ state: 'DENIED', reason: '' }));

  await waitFor(() => {
    expect(utils.current.request.state).toBe('DENIED');
  });
  expect(utils.current.flags).toEqual({
    canAssume: false,
    isAssumed: false,
    canDelete: true,
    canReview: false,
    ownRequest: true,
    isPromoted: false,
  });
});

test('flags for reviewer', async () => {
  const ctx = new TeleportContextE();
  ctx.storeAccessRequests.isAssumed = () => false;
  ctx.storeUser.setState({ ...userContext, username: 'alice' });
  ctx.workflowService.fetchAccessRequest = () =>
    Promise.resolve(requestRolePending);
  accessManagementService.fetchAccessListSuggestions = () =>
    Promise.resolve([]);
  ctx.workflowService.submitAccessRequestReview = () =>
    Promise.resolve({
      ...requestRolePending,
      state: 'APPROVED',
      reviewers: [{ name: 'alice', state: 'APPROVED' as any }],
    });

  const utils = renderHook(() => useRequestView(ctx), {
    wrapper: Wrapper,
  });

  // test on mount setting of flags
  expect(utils.current.user).toBe('alice');
  await waitFor(() => {
    expect(utils.current.flags).toEqual({
      canAssume: false,
      isAssumed: false,
      canDelete: true,
      canReview: true,
      ownRequest: false,
      isPromoted: false,
    });
  });

  // test once reviewed, can't review again
  await act(() =>
    utils.current.submitReview({ state: 'APPROVED', reason: '' })
  );

  await waitFor(() => {
    expect(utils.current.flags).toEqual({
      canAssume: false,
      isAssumed: false,
      canDelete: true,
      canReview: false,
      ownRequest: false,
      isPromoted: false,
    });
  });
});

function Wrapper(props: any) {
  return (
    <MemoryRouter initialEntries={[`web/requests/123`]}>
      <Route path="web/requests/:requestId">{props.children}</Route>
    </MemoryRouter>
  );
}

const userContext = makeUserContext({
  userName: 'Sam',
  userAcl: {
    accessRequests: {
      list: false,
      read: false,
      edit: false,
      create: false,
      remove: true,
    },
  },
  cluster: {
    name: 'aws',
    lastConnected: '2020-09-26T17:30:23.512876876Z',
    status: 'online',
    nodeCount: 1,
    publicURL: 'localhost',
    authVersion: '4.4.0-dev',
    proxyVersion: '4.4.0-dev',
  },
});
