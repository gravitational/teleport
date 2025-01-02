import { RequestView as SharedRequestView } from 'shared/components/AccessRequests/ReviewRequests';
import { useAsync } from 'shared/hooks/useAsync';

import cfg from 'e-teleport/config';
import useTeleportE from 'e-teleport/useTeleportE';
import historyService from 'teleport/services/history';

import useRequestView from './useRequestView';

export function RequestView(props: { requestId: string }) {
  const ctx = useTeleportE();
  const state = useRequestView(ctx);

  const [deleteRequestAttempt, runDeleteRequest] = useAsync(async () => {
    await ctx.workflowService.deleteAccessRequest(props.requestId);
    historyService.replace(cfg.getAccessRequestRoute());
  });

  return (
    <SharedRequestView
      {...state}
      deleteRequestAttempt={deleteRequestAttempt}
      deleteRequest={runDeleteRequest}
    />
  );
}
