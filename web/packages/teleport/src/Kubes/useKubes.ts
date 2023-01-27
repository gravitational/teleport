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
import TeleportContext from 'teleport/teleportContext';
import useStickyClusterId from 'teleport/useStickyClusterId';
import {
  useUrlFiltering,
  useServerSidePagination,
} from 'teleport/components/hooks';

import type { Kube } from 'teleport/services/kube';

export default function useKubes(ctx: TeleportContext) {
  const { clusterId, isLeafCluster } = useStickyClusterId();
  const { username, authType } = ctx.storeUser.state;
  const canCreate = ctx.storeUser.getTokenAccess().create;
  const accessRequestId = ctx.storeUser.getAccessRequestId();
  const { attempt, setAttempt } = useAttempt('processing');
  const [fetchStatus, setFetchStatus] = useState<FetchStatus>('');
  const [results, setResults] = useState<AgentResponse<Kube>>({
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
      fetchFunc: ctx.kubeService.fetchKubernetes,
      clusterId,
      params,
      results,
      setResults,
      setFetchStatus,
      setAttempt,
    });

  useEffect(() => {
    fetchKubes();
  }, [clusterId, search]);

  function fetchKubes() {
    setAttempt({ status: 'processing' });
    ctx.kubeService
      .fetchKubernetes(clusterId, { ...params, limit: pageSize })
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
    username,
    authType,
    isLeafCluster,
    clusterId,
    canCreate,
    results,
    pageSize,
    params,
    fetchStatus,
    accessRequestId,
    ...filteringProps,
    ...paginationProps,
  };
}

export type State = ReturnType<typeof useKubes>;
