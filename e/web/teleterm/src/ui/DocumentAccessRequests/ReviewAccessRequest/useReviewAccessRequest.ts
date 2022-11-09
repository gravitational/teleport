import { useState, useEffect } from 'react';

import { useAppContext } from 'teleterm/ui/appContextProvider';

import { AssumedRequest, LoggedInUser } from 'teleterm/services/tshd/types';
import { AccessRequest, RequestState } from 'e-teleport/services/workflow';
import { useIdentity } from 'teleterm/ui/TopBar/Identity/useIdentity';

import useAttempt from 'shared/hooks/useAttemptNext';
import { retryWithRelogin } from 'teleterm/ui/utils';
import { useWorkspaceContext } from 'teleterm/ui/Documents';

import { makeUiAccessRequest } from '../useAccessRequests';

export default function useReviewAccessRequest({ requestId, goBack }: Props) {
  const ctx = useAppContext();
  ctx.clustersService.useState();

  const { localClusterUri: clusterUri, rootClusterUri } = useWorkspaceContext();
  const identity = useIdentity();
  const [request, setRequest] = useState<AccessRequest>(null);
  const { attempt, run: runGetRequest } = useAttempt('processing');
  const { attempt: submitReviewAttempt, run: runSubmitReview } = useAttempt('');
  const { attempt: deleteRequestAttempt, run: runDeleteRequest } =
    useAttempt('');
  const { attempt: assumeRoleAttempt, run: runAssumeRole } = useAttempt('');
  const [flags, setFlags] = useState<Flags>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const assumed = ctx.clustersService.getAssumedRequests(rootClusterUri);

  const retry = <T>(action: () => Promise<T>) =>
    retryWithRelogin(ctx, clusterUri, action);

  useEffect(() => {
    runGetRequest(() =>
      retry(() =>
        ctx.clustersService
          .getAccessRequest(rootClusterUri, requestId)
          .then(r => {
            const req = makeUiAccessRequest(r);
            setRequest(req);
            setFlags(
              getRequestFlags(
                req,
                identity?.activeRootCluster?.loggedInUser,
                assumed
              )
            );
          })
      )
    );
  }, []);

  useEffect(() => {
    // if workspace assumed object is updated, we update the flags of the current request
    updateFlags();
  }, [assumed]);

  function updateFlags() {
    const user = identity?.activeRootCluster?.loggedInUser;
    if (request && user) {
      setFlags(getRequestFlags(request, user, assumed));
    }
  }

  async function submitReview(state: RequestState, reason: string) {
    const req = {
      state,
      reason,
      roles: request.roles,
      id: request.id,
    };
    runSubmitReview(() =>
      retry(() =>
        ctx.clustersService.reviewAccessRequest(rootClusterUri, req).then(r => {
          const req = makeUiAccessRequest(r);
          setRequest(req);
        })
      )
    );
  }

  async function deleteRequest(requestId: string) {
    runDeleteRequest(() =>
      retry(() =>
        ctx.clustersService.deleteAccessRequest(rootClusterUri, requestId)
      )
    );
  }

  function toggleDeleteConfirmation() {
    setDeleteDialogOpen(!deleteDialogOpen);
  }

  async function assumeRole() {
    runAssumeRole(() =>
      retry(() =>
        // pass the requestId to the requestIds array on its own, and nothing into the dropids array
        // since we are only 'assuming' one requestId at a time
        ctx.clustersService.assumeRole(rootClusterUri, [requestId], [])
      )
    );
  }

  return {
    user: identity.activeRootCluster?.loggedInUser,
    requestId,
    request,
    goBack,
    flags,
    assumeRole,
    attempt,
    submitReviewAttempt,
    assumeRoleAttempt,
    deleteDialogOpen,
    setDeleteDialogOpen,
    deleteRequestAttempt,
    deleteRequest,
    toggleDeleteConfirmation,
    submitReview,
  };
}

function getRequestFlags(
  request: AccessRequest,
  user: LoggedInUser,
  assumedMap: Record<string, AssumedRequest>
) {
  const ownRequest = request.user === user.name;
  const canAssume = ownRequest && request.state === 'APPROVED';
  const isAssumed = !!assumedMap[request.id];
  const canDelete = true;

  const reviewed = request.reviews.find(r => r.author === user.name);

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

type Props = {
  requestId: string;
  goBack: () => void;
};
