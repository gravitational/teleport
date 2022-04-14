import useAttempt from 'shared/hooks/useAttemptNext';
import historyService from 'teleport/services/history';
import TeleportContextE from 'e-teleport/teleportContextE';
import cfg from 'e-teleport/config';

import { State as RequestViewState } from '../RequestView/useRequestView';

export default function useRequestDelete({
  user,
  roles,
  requestId,
  onClose,
  ctx,
}: Props) {
  const { attempt, setAttempt } = useAttempt();

  function onDelete() {
    setAttempt({ status: 'processing' });

    return ctx.workflowService
      .deleteAccessRequest(requestId)
      .then(() => historyService.replace(cfg.getAccessRequestRoute()))
      .catch((err: Error) =>
        setAttempt({ status: 'failed', statusText: err.message })
      );
  }

  return {
    attempt,
    requestId,
    user,
    roles,
    onClose,
    onDelete,
  };
}

export type Props = {
  requestId: string;
  user: string;
  roles: string[];
  onClose: RequestViewState['toggleConfirmDelete'];
  ctx: TeleportContextE;
};
