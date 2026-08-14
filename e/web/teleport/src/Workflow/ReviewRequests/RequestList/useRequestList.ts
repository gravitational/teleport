import { useCallback, useState } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';
import { useKeyBasedPagination } from 'shared/hooks/useInfiniteScroll';
import {
  AccessRequest,
  makeAccessRequest,
} from 'shared/services/accessRequests';

import TeleportContextE from 'e-teleport/teleportContextE';
import { AccessRequestScope, SortType } from 'teleport/services/agents';
import history from 'teleport/services/history';

import { getBaseRequestFlags } from '../requestFlags';

export default function useRequestList(ctx: TeleportContextE) {
  const { attempt, setAttempt } = useAttempt('success');
  const [searchString, setSearchString] = useState('');
  const [scope, setScope] = useState<AccessRequestScope>('');
  const [sortBy, setSortBy] = useState<SortType>({
    fieldName: 'created',
    dir: 'DESC',
  });

  const fetchRequests = useCallback(
    async (params, signal) => {
      const response = await ctx.workflowService.fetchAccessRequests(
        {
          search: searchString || undefined,
          startKey: params.startKey || undefined,
          scope: scope || undefined,
          limit: params.limit,
          sort: `${sortBy.fieldName}:${sortBy.dir}`,
        },
        signal
      );
      return response;
    },
    [searchString, ctx.workflowService, sortBy, scope]
  );

  const {
    fetch,
    resources,
    attempt: fetchAttempt,
    clear,
  } = useKeyBasedPagination({
    fetchFunc: fetchRequests,
    initialFetchSize: 30,
    fetchMoreSize: 30,
    dataKey: 'requests',
  });

  function assumeRole(req: AccessRequestWithFlags) {
    setAttempt({ status: 'processing' });
    ctx.workflowService
      .applyPermission({ requestId: req.id })
      .then(expires => {
        ctx.storeAccessRequests.addAssumed(req, expires);
        history.reload();
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
      });
  }

  function updateScope(scope: AccessRequestScope) {
    setScope(scope);
    clear();
  }

  function updateSort(newSort: SortType) {
    setSortBy(newSort);
    clear();
  }

  return {
    attempt,
    fetch,
    updateSort,
    fetchAttempt,
    resources: resources.map(makeAccessRequest).map(r => makeRow(r, ctx)),
    assumeRole,
    setSearchString,
    scope,
    updateScope,
    searchString,
    sortBy,
    clear,
  };
}

function makeRow(request: AccessRequest, ctx: TeleportContextE) {
  const flags = getBaseRequestFlags(request, ctx);

  return {
    ...request,
    ...flags,
  };
}

export type AccessRequestWithFlags = ReturnType<typeof makeRow>;
export type State = ReturnType<typeof useRequestList>;
