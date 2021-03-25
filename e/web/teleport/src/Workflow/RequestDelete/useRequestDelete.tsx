import useAttempt from 'shared/hooks/useAttemptNext';
import historyService from 'teleport/services/history';
import TeleportContextE from 'e-teleport/teleportContextE';
import cfg from 'e-teleport/config';

import { State as RequestViewState } from '../RequestView/useRequestView';

export default function useRequestDelete({ requestId, onClose, ctx }: Props) {
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
    onClose,
    onDelete,
  };
}

export type Props = {
  requestId: string;
  onClose: RequestViewState['toggleConfirmDelete'];
  ctx: TeleportContextE;
};
