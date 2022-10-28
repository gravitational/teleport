import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import history from 'teleport/services/history';

import TeleportContextE from 'e-teleport/teleportContextE';
import { AccessRequest } from 'e-teleport/services/workflow';
import { usePrivateKeyAccessRequest } from 'e-teleport/hooks/usePrivateKeyRequirement';

export default function useRequestList(ctx: TeleportContextE) {
  const { attempt, run, setAttempt } = useAttempt('processing');
  const [requests, setRequests] = useState<AccessRequest[]>([]);
  const {
    privateKeyRequirement,
    updatePrivateKeyRequirement,
    clearPrivateKeyRequirement,
    isPrivateKeyRequiredError,
  } = usePrivateKeyAccessRequest();

  useEffect(() => {
    run(() =>
      ctx.workflowService.fetchAccessRequests({}).then(reqs => {
        const rows = reqs.map(req => makeRow(req, ctx));
        setRequests(rows);
      })
    );
  }, []);

  function assumeRole(req: Row) {
    setAttempt({ status: 'processing' });
    ctx.workflowService
      .applyPermission({ requestId: req.id })
      .then(expires => {
        ctx.storeAccessRequests.addAssumed(req, expires);
        history.reload();
      })
      .catch((err: Error) => {
        if (isPrivateKeyRequiredError(err)) {
          setAttempt({ status: '' });
          updatePrivateKeyRequirement({
            accessRequestId: req.id,
            authType: ctx.storeUser.state.authType,
            username: req.user,
            clusterId: ctx.storeUser.state.cluster.clusterId,
          });
          return;
        }
        setAttempt({ status: 'failed', statusText: err.message });
      });
  }

  return {
    attempt,
    requests,
    assumeRole,
    privateKeyRequirement,
    clearPrivateKeyRequirement,
  };
}

function makeRow(request: AccessRequest, ctx: TeleportContextE) {
  const ownRequest = request.user === ctx.storeUser.getUsername();
  const canAssume = ownRequest && request.state === 'APPROVED';
  const isAssumed = ownRequest && ctx.storeAccessRequests.isAssumed(request.id);

  return {
    ...request,
    canAssume,
    isAssumed,
  };
}

export type Row = ReturnType<typeof makeRow>;
export type State = ReturnType<typeof useRequestList>;
