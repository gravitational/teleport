import React from 'react';
import useAttempt from 'shared/hooks/useAttempt';
import historyService from 'teleport/services/history';
import { AccessStrategy } from 'teleport/services/user';
import { AccessRequest } from 'e-teleport/services/workflow';
import TeleportContextE from 'e-teleport/teleportContextE';

export default function useWaitingRoom(ctx: TeleportContextE) {
  const workflowService = ctx.workflowService;
  const userService = ctx.userService;
  const accessRequest = ctx.storeAccessRequests.getWaitingRoom();
  const [attempt, attemptActions] = useAttempt({ isProcessing: true });
  const [strategy, setStrategy] = React.useState<AccessStrategy>(null);

  React.useEffect(() => {
    attemptActions.do(() =>
      userService.fetchUserContext().then(res => {
        setStrategy(res.accessStrategy);
        if (
          accessRequest.state === '' &&
          res.accessStrategy.type === 'always'
        ) {
          return createRequest();
        }
      })
    );
  }, []);

  function refresh() {
    return workflowService
      .fetchAccessRequest(accessRequest.id)
      .then(updateState)
      .catch(attemptActions.error);
  }

  function createRequest(reason?: string) {
    return workflowService.createAccessRequest({ reason }).then(updateState);
  }

  function updateState(result: AccessRequest) {
    if (result.state === 'APPROVED') {
      return workflowService.applyPermission(result.id).then(() => {
        ctx.storeAccessRequests.setApprovedWaitingRoom(result);
        historyService.reload();
      });
    }
    ctx.storeAccessRequests.setWaitingRoom(result);
  }

  return {
    attempt,
    accessRequest,
    strategy,
    refresh,
    createRequest,
  };
}

export type State = ReturnType<typeof useWaitingRoom>;
