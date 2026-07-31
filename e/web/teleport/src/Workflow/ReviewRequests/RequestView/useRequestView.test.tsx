import { act, renderHook, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router';

import { requestRolePending } from 'shared/components/AccessRequests/fixtures';

import { accessManagementService } from 'e-teleport/services/accessmanagement';
import TeleportContextE from 'e-teleport/teleportContextE';
import makeUserContext from 'teleport/services/user/makeUserContext';

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

  const { result } = renderHook(() => useRequestView(ctx), {
    wrapper: Wrapper,
  });

  // test on mount, request and flags are init
  await waitFor(() => {
    const accessRequestState =
      result.current.fetchRequestAttempt.status === 'success' &&
      result.current.fetchRequestAttempt.data.state;
    expect(accessRequestState).toBe('PENDING');
  });
  expect(result.current.user).toBe('Sam');
  expect(result.current.userDisplay).toEqual({
    primary: 'Sam Smith',
    secondary: 'sam@example.com',
  });

  expect(result.current.fetchRequestAttempt.data.id).toEqual(
    requestRolePending.id
  );
  expect(
    result.current.getFlags(result.current.fetchRequestAttempt.data)
  ).toEqual({
    canAssume: false,
    isAssumed: false,
    canDelete: true,
    canReview: false,
    ownRequest: true,
    isPromoted: false,
  });

  // test setting of request and flags after request is approved
  await act(() =>
    result.current.submitReview({ state: 'APPROVED', reason: '' })
  );
  await waitFor(() => {
    const accessRequestState =
      result.current.submitReviewAttempt.status === 'success' &&
      result.current.submitReviewAttempt.data.state;
    expect(accessRequestState).toBe('APPROVED');
  });
  expect(
    result.current.getFlags(result.current.submitReviewAttempt.data)
  ).toEqual({
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
    result.current.submitReview({ state: 'APPROVED', reason: '' })
  );

  await waitFor(() => {
    expect(
      result.current.getFlags(result.current.submitReviewAttempt.data)
    ).toEqual({
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
  await act(() => result.current.submitReview({ state: 'DENIED', reason: '' }));

  await waitFor(() => {
    const accessRequestState =
      result.current.submitReviewAttempt.status === 'success' &&
      result.current.submitReviewAttempt.data.state;
    expect(accessRequestState).toBe('DENIED');
  });
  expect(
    result.current.getFlags(result.current.submitReviewAttempt.data)
  ).toEqual({
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

  const { result } = renderHook(() => useRequestView(ctx), {
    wrapper: Wrapper,
  });

  // test on mount setting of flags
  expect(result.current.user).toBe('alice');
  await waitFor(() => {
    expect(result.current.fetchRequestAttempt.status).toBe('success');
  });
  expect(
    result.current.getFlags(result.current.fetchRequestAttempt.data)
  ).toEqual({
    canAssume: false,
    isAssumed: false,
    canDelete: true,
    canReview: true,
    ownRequest: false,
    isPromoted: false,
  });

  // test once reviewed, can't review again
  await act(() =>
    result.current.submitReview({ state: 'APPROVED', reason: '' })
  );

  await waitFor(() => {
    expect(result.current.submitReviewAttempt.status).toBe('success');
  });
  expect(
    result.current.getFlags(result.current.submitReviewAttempt.data)
  ).toEqual({
    canAssume: false,
    isAssumed: false,
    canDelete: true,
    canReview: false,
    ownRequest: false,
    isPromoted: false,
  });
});

function Wrapper(props: any) {
  return (
    <MemoryRouter initialEntries={[`/web/requests/123`]}>
      <Routes>
        <Route path="/web/requests/:requestId" element={props.children} />
      </Routes>
    </MemoryRouter>
  );
}

const userContext = makeUserContext({
  userName: 'Sam',
  displayPrimary: 'Sam Smith',
  displaySecondary: 'sam@example.com',
  userAcl: {
    reviewRequests: true,
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
