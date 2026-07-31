import { useCallback, useEffect, useState } from 'react';
import { useParams } from 'react-router';

import type {
  RequestFlags,
  SubmitReview,
} from 'shared/components/AccessRequests/ReviewRequests';
import { useAsync } from 'shared/hooks/useAsync';

import { accessManagementService } from 'e-teleport/services/accessmanagement';
import { AccessRequest } from 'e-teleport/services/workflow';
import TeleportContextE from 'e-teleport/teleportContextE';
import history from 'teleport/services/history';
import session from 'teleport/services/websession';

import { getBaseRequestFlags } from '../requestFlags';

export default function useRequestView(ctx: TeleportContextE) {
  const { requestId = '' } = useParams<{ requestId: string }>();

  const [fetchRequestAttempt, runFetchRequest] = useAsync(
    useCallback(
      () => ctx.workflowService.fetchAccessRequest(requestId),
      [ctx.workflowService, requestId]
    )
  );
  const [fetchSuggestedAccessListsAttempt, runFetchSuggestedAccessLists] =
    useAsync(
      useCallback(
        () => accessManagementService.fetchAccessListSuggestions(requestId),
        [requestId]
      )
    );
  const [submitReviewAttempt, runSubmitReview] = useAsync(
    async (review: SubmitReview) => {
      // This should not happen because the UI is hidden when fetching the request is in progress.
      if (fetchRequestAttempt.status !== 'success') {
        throw new Error('No access request to review.');
      }

      return review.state === 'PROMOTED' && review.promotedToAccessList
        ? await ctx.workflowService.promoteAccessRequest(requestId, {
            accessListName: review.promotedToAccessList.id,
            reason: review.reason,
          })
        : await ctx.workflowService.submitAccessRequestReview({
            state: review.state,
            reason: review.reason,
            roles: fetchRequestAttempt.data.roles,
            id: requestId,
            assumeStartTime: review.assumeStartTime,
          });
    }
  );
  const [assumeRoleAttempt, runAssumeRole] = useAsync(
    async (request: AccessRequest) => {
      const expires = await ctx.workflowService.applyPermission({
        requestId: request.id,
      });
      ctx.storeAccessRequests.addAssumed(request, expires);
      history.reload();
    }
  );

  const [confirmDelete, setConfirmDelete] = useState(false);

  function getFlags(accessRequest: AccessRequest): RequestFlags {
    return getRequestFlags(accessRequest, ctx);
  }

  useEffect(() => {
    runFetchRequest();
    runFetchSuggestedAccessLists();
  }, [runFetchRequest, runFetchSuggestedAccessLists]);

  function toggleConfirmDelete(): void {
    setConfirmDelete(!confirmDelete);
  }

  return {
    user: ctx.storeUser.getUsername(),
    userDisplay: {
      primary: ctx.storeUser.state.displayPrimary,
      secondary: ctx.storeUser.state.displaySecondary,
    },
    fetchRequestAttempt,
    getFlags,
    confirmDelete,
    toggleConfirmDelete,
    submitReview: runSubmitReview,
    submitReviewAttempt,
    assumeRole: runAssumeRole,
    assumeRoleAttempt,
    fetchSuggestedAccessListsAttempt,
    assumeAccessList: session.logout,
  };
}

function getRequestFlags(
  request: AccessRequest,
  ctx: TeleportContextE
): RequestFlags {
  const flags = getBaseRequestFlags(request, ctx);
  const canDelete = ctx.storeUser.getWorkflowAccess().remove;

  const reviewed = request.reviewers.find(
    r => r.name === ctx.storeUser.getUsername()
  );

  const isPendingState = reviewed
    ? reviewed.state === 'PENDING'
    : request.state === 'PENDING';

  const canReview =
    ctx.storeUser.getReviewRequests() && !flags.ownRequest && isPendingState;

  return {
    ...flags,
    canDelete,
    canReview,
  };
}
