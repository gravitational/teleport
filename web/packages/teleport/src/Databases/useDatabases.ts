/*
Copyright 2021-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { useState, useEffect } from 'react';
import { FetchStatus } from 'design/DataTable/types';
import useAttempt from 'shared/hooks/useAttemptNext';

import { AgentResponse } from 'teleport/services/agents';
import Ctx from 'teleport/teleportContext';
import useStickyClusterId from 'teleport/useStickyClusterId';
import {
  useUrlFiltering,
  useServerSidePagination,
} from 'teleport/components/hooks';

import type { Database } from 'teleport/services/databases';

export default function useDatabases(ctx: Ctx) {
  const { attempt, setAttempt } = useAttempt('processing');
  const { clusterId, isLeafCluster } = useStickyClusterId();
  const username = ctx.storeUser.state.username;
  const canCreate = ctx.storeUser.getTokenAccess().create;
  const authType = ctx.storeUser.state.authType;
  const accessRequestId = ctx.storeUser.getAccessRequestId();
  const [fetchStatus, setFetchStatus] = useState<FetchStatus>('');
  const [results, setResults] = useState<AgentResponse<Database>>({
    agents: [],
    startKey: '',
    totalCount: 0,
  });

  const { params, search, ...filteringProps } = useUrlFiltering({
    fieldName: 'name',
    dir: 'ASC',
  });

  const { setStartKeys, pageSize, ...paginationProps } =
    useServerSidePagination({
      fetchFunc: ctx.databaseService.fetchDatabases,
      clusterId,
      params,
      results,
      setResults,
      setFetchStatus,
      setAttempt,
    });

  useEffect(() => {
    fetchDatabases();
  }, [clusterId, search]);

  function fetchDatabases() {
    setAttempt({ status: 'processing' });
    ctx.databaseService
      .fetchDatabases(clusterId, { ...params, limit: pageSize })
      .then(res => {
        setResults(res);
        setFetchStatus(res.startKey ? '' : 'disabled');
        setStartKeys(['', res.startKey]);
        setAttempt({ status: 'success' });
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
        setResults({ ...results, agents: [], totalCount: 0 });
        setStartKeys(['']);
      });
  }

  return {
    attempt,
    canCreate,
    isLeafCluster,
    username,
    clusterId,
    authType,
    results,
    pageSize,
    params,
    fetchStatus,
    accessRequestId,
    ...filteringProps,
    ...paginationProps,
  };
}

export type State = ReturnType<typeof useDatabases>;
