import useAttempt from 'shared/hooks/useAttemptNext';
import historyService from 'teleport/services/history';
import TeleportContextE from 'e-teleport/teleportContextE';
import cfg from 'e-teleport/config';

import { State as RequestViewState } from '../useRequestView';

import type { RequestState } from 'e-teleport/services/workflow';

export default function useRequestDelete({
  user,
  roles,
  requestId,
  requestState,
  onClose,
  ctx,
}: Props) {
  const { attempt, setAttempt } = useAttempt();

  function onDelete() {
    setAttempt({ status: 'processing' });

    return ctx.workflowService
      .deleteAccessRequest(requestId)
      .then(() => {
        historyService.replace(cfg.getAccessRequestRoute());
      })
      .catch((err: Error) =>
        setAttempt({ status: 'failed', statusText: err.message })
      );
  }

  return {
    attempt,
    requestId,
    requestState,
    user,
    roles,
    onClose,
    onDelete,
  };
}

export type Props = {
  requestId: string;
  requestState: RequestState;
  user: string;
  roles: string[];
  onClose: RequestViewState['toggleConfirmDelete'];
  ctx: TeleportContextE;
  clusterId?: string;
};
