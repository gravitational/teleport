import React from 'react';
import { MemoryRouter, Route } from 'react-router';
import useRequestView from './useRequestView';
import renderHook, { act } from 'design/utils/renderHook';
import makeUserContext from 'teleport/services/user/makeUserContext';
import TeleportContextE from 'e-teleport/teleportContextE';
import { requestPending } from 'e-teleport/Workflow/fixtures';

test('flags for own request', async () => {
  const ctx = new TeleportContextE();
  ctx.storeUser.setState({ ...userContext });
  ctx.storeAccessRequests.isAssumed = () => false;
  ctx.workflowService.fetchAccessRequest = () =>
    Promise.resolve(requestPending);
  ctx.workflowService.submitAccessRequestReview = () =>
    Promise.resolve({ ...requestPending, state: 'APPROVED' });

  let hook;
  await act(async () => {
    hook = renderHook(() => useRequestView(ctx), {
      wrapper: Wrapper,
    });
  });

  // test on mount, request and flags are init
  expect(hook.current.user).toBe('Sam');
  expect(hook.current.request.id).toEqual(requestPending.id);
  expect(hook.current.request.state).toBe('PENDING');
  expect(hook.current.flags).toEqual({
    canAssume: false,
    isAssumed: false,
    canDelete: true,
    canReview: false,
  });

  // test setting of request and flags after request is approved
  await act(async () => hook.current.submitReview('APPROVED', ''));
  expect(hook.current.request.state).toBe('APPROVED');
  expect(hook.current.flags).toEqual({
    canAssume: true,
    isAssumed: false,
    canDelete: true,
    canReview: false,
  });

  // test review isAssumed flag
  ctx.storeAccessRequests.isAssumed = () => true;
  await act(async () => hook.current.submitReview('APPROVED', ''));
  expect(hook.current.flags).toEqual({
    canAssume: true,
    isAssumed: true,
    canDelete: true,
    canReview: false,
  });

  // test setting of request and flags after request is denied
  ctx.storeAccessRequests.isAssumed = () => false;
  ctx.workflowService.submitAccessRequestReview = () =>
    Promise.resolve({ ...requestPending, state: 'DENIED' });
  await act(async () => hook.current.submitReview('DENIED', ''));
  expect(hook.current.request.state).toBe('DENIED');
  expect(hook.current.flags).toEqual({
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
    Promise.resolve(requestPending);
  ctx.workflowService.submitAccessRequestReview = () =>
    Promise.resolve({
      ...requestPending,
      state: 'APPROVED',
      reviewers: [{ name: 'alice', state: 'APPROVED' as any }],
    });

  let hook;
  await act(async () => {
    hook = renderHook(() => useRequestView(ctx), {
      wrapper: Wrapper,
    });
  });

  // test on mount setting of flags
  expect(hook.current.user).toBe('alice');
  expect(hook.current.flags).toEqual({
    canAssume: false,
    isAssumed: false,
    canDelete: true,
    canReview: true,
  });

  // test once reviewed, can't review again
  await act(async () => hook.current.submitReview('APPROVED', ''));
  expect(hook.current.flags).toEqual({
    canAssume: false,
    isAssumed: false,
    canDelete: true,
    canReview: false,
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
