import { useState, useEffect } from 'react';
import { useParams } from 'react-router';
import useAttempt from 'shared/hooks/useAttemptNext';
import history from 'teleport/services/history';
import { getErrMessage } from 'shared/utils/errorType';

import TeleportContextE from 'e-teleport/teleportContextE';
import {
  AccessRequest,
  PromoteAccessRequest,
  UpdateAccessRequest,
} from 'e-teleport/services/workflow';
import { usePrivateKeyAccessRequest } from 'e-teleport/hooks/usePrivateKeyRequirement';
import { accessManagementService } from 'e-teleport/services/accessmanagement';
import { getBaseRequestFlags } from 'e-teleport/Workflow/Shared';

import type { Attempt } from 'shared/hooks/useAttemptNext';
import type { PrivateKeyAccessRequest } from 'teleport/components/PrivateKeyPolicy';

import type { LongTermAccess, SubmitReview } from './types';

export default function useRequestView(ctx: TeleportContextE) {
  const { requestId } = useParams<{ requestId: string }>();

  const { attempt, setAttempt } = useAttempt('processing');
  const reviewAttempt = useAttempt();

  const [request, setRequest] = useState<AccessRequest>(null);
  const [longTermAccess, setLongTermAccess] = useState<LongTermAccess>({
    suggestedAccessLists: [],
    error: '',
  });

  const [confirmDelete, setConfirmDelete] = useState(false);
  const [flags, setFlags] = useState<Flags>(null);
  const {
    privateKeyRequirement,
    updatePrivateKeyRequirement,
    clearPrivateKeyRequirement,
    isPrivateKeyRequiredError,
  } = usePrivateKeyAccessRequest();

  useEffect(() => {
    async function initialFetch() {
      try {
        const fetchedAccessRequest =
          await ctx.workflowService.fetchAccessRequest(requestId);

        setRequest(fetchedAccessRequest);
        setFlags(getRequestFlags(fetchedAccessRequest, ctx));

        // Skip fetching access list suggestions for non-pending
        // requests.
        if (fetchedAccessRequest.state !== 'PENDING') {
          setAttempt({ status: 'success' });
          return;
        }
      } catch (err) {
        const errMsg = getErrMessage(err);
        setAttempt({ status: 'failed', statusText: errMsg });

        // Don't try to fetch access list suggestions.
        return;
      }

      try {
        const suggestedAccessLists =
          await accessManagementService.fetchAccessListSuggestions(requestId);

        setLongTermAccess({ suggestedAccessLists, error: '' });
      } catch (err) {
        const errMsg = getErrMessage(err);
        setLongTermAccess({ suggestedAccessLists: [], error: errMsg });
      }

      setAttempt({ status: 'success' });
    }

    initialFetch();
  }, []);

  async function submitReview({
    state,
    reason,
    promotedToAccessList,
  }: SubmitReview) {
    if (state === 'PROMOTED' && promotedToAccessList) {
      const promoteReq: PromoteAccessRequest = {
        accessListName: promotedToAccessList.id,
        reason,
      };

      reviewAttempt.run(() =>
        ctx.workflowService
          .promoteAccessRequest(request.id, promoteReq)
          .then(req => {
            setRequest(req);
            setFlags(getRequestFlags(req, ctx));
          })
      );

      return;
    }

    const reviewReq: UpdateAccessRequest = {
      state,
      reason,
      roles: request.roles,
      id: request.id,
    };

    reviewAttempt.run(() =>
      ctx.workflowService.submitAccessRequestReview(reviewReq).then(req => {
        setRequest(req);
        setFlags(getRequestFlags(req, ctx));
      })
    );
  }

  function toggleConfirmDelete() {
    setConfirmDelete(!confirmDelete);
  }

  function assumeRole() {
    setAttempt({ status: 'processing' });
    ctx.workflowService
      .applyPermission({ requestId: request.id })
      .then(expires => {
        ctx.storeAccessRequests.addAssumed(request, expires);
        history.reload();
      })
      .catch((err: Error) => {
        if (isPrivateKeyRequiredError(err)) {
          setAttempt({ status: '' });
          updatePrivateKeyRequirement({
            accessRequestId: request.id,
            authType: ctx.storeUser.state.authType,
            username: request.user,
            clusterId: ctx.storeUser.state.cluster.clusterId,
          });
          return;
        }
        setAttempt({ status: 'failed', statusText: err.message });
      });
  }

  return {
    user: ctx.storeUser.getUsername(),
    reviewAttempt: reviewAttempt.attempt,
    attempt,
    request,
    flags,
    confirmDelete,
    toggleConfirmDelete,
    submitReview,
    assumeRole,
    privateKeyRequirement,
    clearPrivateKeyRequirement,
    longTermAccess,
  };
}

function getRequestFlags(request: AccessRequest, ctx: TeleportContextE) {
  const flags = getBaseRequestFlags(request, ctx);
  const canDelete = ctx.storeUser.getWorkflowAccess().remove;

  const reviewed = request.reviewers.find(
    r => r.name === ctx.storeUser.getUsername()
  );

  const isPendingState = reviewed
    ? reviewed.state === 'PENDING'
    : request.state === 'PENDING';

  return {
    ...flags,
    canDelete,
    canReview: !flags.ownRequest && isPendingState,
  };
}

type Flags = ReturnType<typeof getRequestFlags>;

export type State = {
  user: string;
  reviewAttempt: Attempt;
  attempt: Attempt;
  request: AccessRequest;
  flags: Flags;
  confirmDelete: boolean;
  toggleConfirmDelete(): void;
  submitReview(s: SubmitReview);
  assumeRole(): void;
  privateKeyRequirement?: PrivateKeyAccessRequest;
  clearPrivateKeyRequirement?(): void;
  longTermAccess: LongTermAccess;
};
