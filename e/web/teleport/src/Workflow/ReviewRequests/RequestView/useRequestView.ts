import { useState, useEffect } from 'react';
import { useParams } from 'react-router';
import useAttempt from 'shared/hooks/useAttemptNext';
import useWebSession from 'teleport/useWebSession';
import history from 'teleport/services/history';

import TeleportContextE from 'e-teleport/teleportContextE';
import { AccessRequest, RequestState } from 'e-teleport/services/workflow';
import { usePrivateKeyAccessRequest } from 'e-teleport/hooks/usePrivateKeyRequirement';

import type { Attempt } from 'shared/hooks/useAttemptNext';
import type { PrivateKeyAccessRequest } from 'teleport/components/PrivateKeyPolicy';

export default function useRequestView(ctx: TeleportContextE) {
  const webSession = useWebSession();
  const { requestId } = useParams<{ requestId: string }>();

  const { attempt, setAttempt, run } = useAttempt('processing');
  const reviewAttempt = useAttempt();

  const [request, setRequest] = useState<AccessRequest>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [flags, setFlags] = useState<Flags>(null);
  const {
    privateKeyRequirement,
    updatePrivateKeyRequirement,
    clearPrivateKeyRequirement,
    isPrivateKeyRequiredError,
  } = usePrivateKeyAccessRequest();

  useEffect(() => {
    run(() =>
      ctx.workflowService.fetchAccessRequest(requestId).then(req => {
        setRequest(req);
        setFlags(getRequestFlags(req, ctx));
      })
    );
  }, []);

  function submitReview(state: RequestState, reason: string) {
    const req = {
      state,
      reason,
      roles: request.roles,
      id: request.id,
    };

    reviewAttempt.run(() =>
      ctx.workflowService.submitAccessRequestReview(req).then(req => {
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
      .applyPermission({ requestId: request.id }, webSession)
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
  };
}

function getRequestFlags(request: AccessRequest, ctx: TeleportContextE) {
  const ownRequest = request.user === ctx.storeUser.getUsername();
  const canAssume = ownRequest && request.state === 'APPROVED';
  const isAssumed = ownRequest && ctx.storeAccessRequests.isAssumed(request.id);
  const canDelete = ctx.storeUser.getWorkflowAccess().remove;

  const reviewed = request.reviewers.find(
    r => r.name === ctx.storeUser.getUsername()
  );

  const isPendingState = reviewed
    ? reviewed.state === 'PENDING'
    : request.state === 'PENDING';

  return {
    // canAssume is a flag to show the assume btn.
    canAssume,
    // isAssumed is a flag if the assume btn should be disabled or not,
    // and determines the text that implies if user already has assumed or not.
    isAssumed,
    canDelete,
    canReview: !ownRequest && isPendingState,
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
  submitReview(requestState: RequestState, reason: string);
  assumeRole(): void;
  privateKeyRequirement?: PrivateKeyAccessRequest;
  clearPrivateKeyRequirement?(): void;
};
