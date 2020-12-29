import React from 'react';
import { useHistory } from 'react-router';
import useAttempt from 'shared/hooks/useAttemptNext';
import { Option } from 'shared/components/Select';
import TeleportContextE from 'e-teleport/teleportContextE';
import cfg from 'e-teleport/config';

export default function useRequestCreate(ctx: TeleportContextE) {
  const history = useHistory();
  const roles = ctx.storeUser.getRequestableRoles();
  const requireReason = ctx.storeUser.getAccessStrategy().type === 'reason';
  const { attempt, setAttempt } = useAttempt();
  const [reason, setReason] = React.useState('');
  const [selectedRoles, setSelectedRoles] = React.useState<Option[]>([]);

  function createRequest() {
    const request = {
      reason,
      roles: selectedRoles.map(r => r.value),
    };

    setAttempt({ status: 'processing' });
    ctx.workflowService
      .createAccessRequest(request)
      .then(close)
      .catch((err: Error) =>
        setAttempt({ status: 'failed', statusText: err.message })
      );
  }

  function close() {
    history.push(cfg.routes.requests);
  }

  return {
    attempt,
    requireReason,
    reason,
    setReason,
    roles,
    selectedRoles,
    setSelectedRoles,
    createRequest,
    close,
  };
}

export type State = ReturnType<typeof useRequestCreate>;
