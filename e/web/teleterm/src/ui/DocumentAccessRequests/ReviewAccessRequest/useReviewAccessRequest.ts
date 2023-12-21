import { useState, useEffect, useCallback } from 'react';

import { AccessRequest } from 'e-teleport/services/workflow';
import {
  SubmitReview,
  RequestFlags,
} from 'e-teleport/Workflow/ReviewRequests/RequestView/types';

import { AssumedRequest, LoggedInUser } from 'teleterm/services/tshd/types';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceLoggedInUser } from 'teleterm/ui/hooks/useLoggedInUser';
import { retryWithRelogin } from 'teleterm/ui/utils';
import { useWorkspaceContext } from 'teleterm/ui/Documents';

import { useAsync } from 'shared/hooks/useAsync';

import { makeUiAccessRequest } from '../useAccessRequests';

export function useReviewAccessRequest({
  requestId,
  goBack,
}: {
  requestId: string;
  goBack(): void;
}) {
  const ctx = useAppContext();
  ctx.clustersService.useState();

  const { localClusterUri: clusterUri, rootClusterUri } = useWorkspaceContext();
  const loggedInUser = useWorkspaceLoggedInUser();
  const assumed = ctx.clustersService.getAssumedRequests(rootClusterUri);

  const retry = useCallback(
    <T>(action: () => Promise<T>) => retryWithRelogin(ctx, clusterUri, action),
    [clusterUri, ctx]
  );

  const [fetchRequestAttempt, runFetchRequest] = useAsync(
    useCallback(
      () =>
        retry(async () => {
          const request = await ctx.clustersService.getAccessRequest(
            rootClusterUri,
            requestId
          );
          return makeUiAccessRequest(request);
        }),
      [ctx.clustersService, requestId, retry, rootClusterUri]
    )
  );
  const [deleteRequestAttempt, runDeleteRequest] = useAsync(() =>
    retry(() =>
      ctx.clustersService.deleteAccessRequest(rootClusterUri, requestId)
    )
  );
  const [assumeRoleAttempt, runAssumeRole] = useAsync(() =>
    retry(() => ctx.clustersService.assumeRole(rootClusterUri, [requestId], []))
  );
  const [submitReviewAttempt, runSubmitReview] = useAsync(
    (review: SubmitReview) =>
      retry(async () => {
        // This should not happen because the UI is hidden when fetching the request is in progress.
        if (fetchRequestAttempt.status !== 'success') {
          throw new Error('No access request to review.');
        }

        const updatedAccessRequest =
          await ctx.clustersService.reviewAccessRequest(rootClusterUri, {
            state: review.state,
            reason: review.reason,
            roles: fetchRequestAttempt.data.roles,
            id: requestId,
          });

        return makeUiAccessRequest(updatedAccessRequest);
      })
  );

  function getFlags(request: AccessRequest): RequestFlags {
    if (loggedInUser) {
      return getRequestFlags(request, loggedInUser, assumed);
    }
    return undefined;
  }

  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  useEffect(() => {
    if (fetchRequestAttempt.status === '') {
      runFetchRequest();
    }
  }, [fetchRequestAttempt.status, runFetchRequest]);

  async function deleteRequest(): Promise<void> {
    const [, error] = await runDeleteRequest();
    if (!error) {
      goBack();
    }
  }

  return {
    user: loggedInUser,
    getFlags,
    assumeRole: runAssumeRole,
    fetchRequestAttempt,
    submitReviewAttempt,
    assumeRoleAttempt,
    deleteDialogOpen,
    setDeleteDialogOpen,
    deleteRequestAttempt,
    deleteRequest,
    submitReview: runSubmitReview,
  };
}

function getRequestFlags(
  request: AccessRequest,
  user: LoggedInUser,
  assumedMap: Record<string, AssumedRequest>
): RequestFlags {
  const ownRequest = request.user === user.name;
  const canAssume = ownRequest && request.state === 'APPROVED';
  const isAssumed = !!assumedMap[request.id];
  const canDelete = true;

  const reviewed = request.reviews.find(r => r.author === user.name);

  const isPromoted = request.state === 'PROMOTED';

  const isPendingState = reviewed
    ? reviewed.state === 'PENDING'
    : request.state === 'PENDING';

  return {
    canAssume,
    isAssumed,
    canDelete,
    canReview: !ownRequest && isPendingState,
    isPromoted,
    ownRequest,
  };
}
