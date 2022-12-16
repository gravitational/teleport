import React from 'react';
import { MemoryRouter, Route } from 'react-router';
import { waitFor } from '@testing-library/react';

import renderHook, { act } from 'design/utils/renderHook';
import makeUserContext from 'teleport/services/user/makeUserContext';
import { getMockWebSession } from 'teleport/services/websession/test-utils';
import { SessionContextProvider } from 'teleport/WebSessionContext';

import TeleportContextE from 'e-teleport/teleportContextE';
import { requestRolePending } from 'e-teleport/Workflow/fixtures';

import useRequestView from './useRequestView';

test('flags for own request', async () => {
  const ctx = new TeleportContextE();
  ctx.storeUser.setState(userContext);
  ctx.storeAccessRequests.isAssumed = () => false;
  ctx.workflowService.fetchAccessRequest = () =>
    Promise.resolve(requestRolePending);
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
  });

  // test setting of request and flags after request is approved
  act(() => utils.current.submitReview('APPROVED', ''));
  await waitFor(() => {
    expect(utils.current.request.state).toBe('APPROVED');
  });
  expect(utils.current.flags).toEqual({
    canAssume: true,
    isAssumed: false,
    canDelete: true,
    canReview: false,
  });

  // test review isAssumed flag
  ctx.storeAccessRequests.isAssumed = () => true;
  act(() => utils.current.submitReview('APPROVED', ''));

  await waitFor(() => {
    expect(utils.current.flags).toEqual({
      canAssume: true,
      isAssumed: true,
      canDelete: true,
      canReview: false,
    });
  });

  // test setting of request and flags after request is denied
  ctx.storeAccessRequests.isAssumed = () => false;
  ctx.workflowService.submitAccessRequestReview = () =>
    Promise.resolve({ ...requestRolePending, state: 'DENIED' });
  act(() => utils.current.submitReview('DENIED', ''));

  await waitFor(() => {
    expect(utils.current.request.state).toBe('DENIED');
  });
  expect(utils.current.flags).toEqual({
    canAssume: false,
    isAssumed: false,
    canDelete: true,
    canReview: false,
  });
});

test('flags for reviewer', async () => {
  const ctx = new TeleportContextE();
  ctx.storeAccessRequests.isAssumed = () => false;
  ctx.storeUser.setState({ ...userContext, username: 'alice' });
  ctx.workflowService.fetchAccessRequest = () =>
    Promise.resolve(requestRolePending);
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
    });
  });

  // test once reviewed, can't review again
  act(() => utils.current.submitReview('APPROVED', ''));

  await waitFor(() => {
    expect(utils.current.flags).toEqual({
      canAssume: false,
      isAssumed: false,
      canDelete: true,
      canReview: false,
    });
  });
});

function Wrapper(props: any) {
  const mockWebSession = getMockWebSession();

  return (
    <MemoryRouter initialEntries={[`web/requests/123`]}>
      <SessionContextProvider session={mockWebSession}>
        <Route path="web/requests/:requestId">{props.children}</Route>
      </SessionContextProvider>
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
