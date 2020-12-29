import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import historyService from 'teleport/services/history';

import TeleportContextE from 'e-teleport/teleportContextE';
import { AccessRequest } from 'e-teleport/services/workflow';

export default function useRequestList(ctx: TeleportContextE) {
  const currUser = ctx.storeUser.getUsername();
  const { attempt, run, setAttempt } = useAttempt('processing');
  const [requests, setRequests] = useState<AccessRequest[]>(null);

  useEffect(() => {
    run(() =>
      ctx.workflowService.fetchAccessRequests({ user: currUser }).then(reqs => {
        const rows = reqs.map(req => makeRow(req, ctx));
        setRequests(rows);
      })
    );
  }, []);

  function assumeRole(req: Row) {
    setAttempt({ status: 'processing' });
    ctx.workflowService
      .applyPermission(req.id)
      .then(() => {
        ctx.storeAccessRequests.addAssumed(req);
        historyService.reload();
      })
      .catch((err: Error) =>
        setAttempt({ status: 'failed', statusText: err.message })
      );
  }

  return {
    attempt,
    requests,
    assumeRole,
  };
}

function makeRow(request: AccessRequest, ctx: TeleportContextE) {
  const canAssume = request.state === 'APPROVED';
  const isAssumed = ctx.storeAccessRequests.isAssumed(request.id);

  return {
    ...request,
    canAssume,
    isAssumed,
  };
}

export type Row = ReturnType<typeof makeRow>;
export type State = ReturnType<typeof useRequestList>;
