import React from 'react';
import { useHistory } from 'react-router';
import useAttempt from 'shared/hooks/useAttemptNext';
import TeleportContextE from 'e-teleport/teleportContextE';
import cfg from 'e-teleport/config';

export default function useRequestCreate(ctx: TeleportContextE) {
  const history = useHistory();
  const roles = ctx.storeUser.getRequestableRoles();
  const reviewers = ctx.storeUser.getSuggestedReviewers();
  const requireReason = ctx.storeUser.getAccessStrategy().type === 'reason';
  const { attempt, setAttempt } = useAttempt();
  const [reason, setReason] = React.useState('');

  function createRequest(roles: string[], suggestedReviewers: string[]) {
    setAttempt({ status: 'processing' });
    ctx.workflowService
      .createAccessRequest({ reason, roles, suggestedReviewers })
      .then(close)
      .catch((err: Error) =>
        setAttempt({ status: 'failed', statusText: err.message })
      );
  }

  function close() {
    history.push(cfg.getAccessRequestRoute());
  }

  return {
    attempt,
    requireReason,
    reason,
    setReason,
    roles,
    reviewers,
    createRequest,
    close,
  };
}

export type State = ReturnType<typeof useRequestCreate>;
