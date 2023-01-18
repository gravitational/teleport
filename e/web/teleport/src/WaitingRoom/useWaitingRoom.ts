import React from 'react';
import useAttempt from 'shared/hooks/useAttempt';
import historyService from 'teleport/services/history';
import { UserContext } from 'teleport/services/user';
import cfg from 'teleport/config';

import { AccessRequest } from 'e-teleport/services/workflow';
import TeleportContextE from 'e-teleport/teleportContextE';
import { usePrivateKeyAccessRequest } from 'e-teleport/hooks/usePrivateKeyRequirement';

export default function useWaitingRoom(ctx: TeleportContextE) {
  const workflowService = ctx.workflowService;
  const userService = ctx.userService;
  const accessRequest = ctx.storeAccessRequests.getWaitingRoom();
  const [attempt, attemptActions] = useAttempt({ isProcessing: true });
  const [userCtx, setUserCtx] = React.useState<UserContext>();
  const {
    privateKeyRequirement,
    updatePrivateKeyRequirement,
    clearPrivateKeyRequirement,
    isPrivateKeyRequiredError,
  } = usePrivateKeyAccessRequest();

  React.useEffect(() => {
    attemptActions.do(() =>
      userService.fetchUserContext().then(res => {
        // User can only assume roles from the UI, if they have access to the dashboard.
        // Since waiting room is used for initial login only, this check prevents the user
        // from going into the waiting room if the role they assumed (from an approved
        // access request) has the waiting room enabled.
        if (ctx.storeAccessRequests.getAssumedRoles().length > 0) {
          return;
        }

        setUserCtx(res);
        // This statement says: on login, if the strategy is always, auto create a request for user.
        // An access request state is retrieved from local storage and is unitialized on logins.
        // (logging out and session expiry clears the storage).
        if (!accessRequest.state && res.accessStrategy.type === 'always') {
          return createRequest();
        }
      })
    );
  }, []);

  function refresh() {
    return workflowService
      .fetchAccessRequest(accessRequest.id)
      .then(updateState)
      .catch((err: Error) => {
        if (isPrivateKeyRequiredError(err)) {
          attemptActions.clear();
          updatePrivateKeyRequirement({
            accessRequestId: accessRequest.id,
            authType: userCtx?.authType,
            username: accessRequest.user,
            clusterId: cfg.proxyCluster,
          });

          return;
        }
        attemptActions.error(err);
      });
  }

  function createRequest(reason?: string) {
    return workflowService.createAccessRequest({ reason }).then(updateState);
  }

  function updateState(result: AccessRequest) {
    if (result.state === 'APPROVED') {
      return workflowService
        .applyPermission({ requestId: result.id })
        .then(expires => {
          ctx.storeAccessRequests.setApprovedWaitingRoom(result, expires);
          historyService.reload();
        });
    }
    ctx.storeAccessRequests.setWaitingRoom(result);
  }

  return {
    attempt,
    accessRequest,
    strategy: userCtx?.accessStrategy,
    refresh,
    createRequest,
    privateKeyRequirement,
    clearPrivateKeyRequirement,
  };
}

export type State = ReturnType<typeof useWaitingRoom>;
