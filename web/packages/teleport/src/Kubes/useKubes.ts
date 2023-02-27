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

import { useEffect } from 'react';

import TeleportContext from 'teleport/teleportContext';
import useStickyClusterId from 'teleport/useStickyClusterId';
import {
  useUrlFiltering,
  useServerSidePagination,
} from 'teleport/components/hooks';

export function useKubes(ctx: TeleportContext) {
  const { clusterId, isLeafCluster } = useStickyClusterId();
  const { username, authType } = ctx.storeUser.state;
  const canCreate = ctx.storeUser.getTokenAccess().create;
  const accessRequestId = ctx.storeUser.getAccessRequestId();

  const { params, search, ...filteringProps } = useUrlFiltering({
    fieldName: 'name',
    dir: 'ASC',
  });

  const { fetch, ...paginationProps } = useServerSidePagination({
    fetchFunc: ctx.kubeService.fetchKubernetes,
    clusterId,
    params,
  });

  useEffect(() => {
    fetch();
  }, [clusterId, search]);

  return {
    username,
    authType,
    isLeafCluster,
    clusterId,
    canCreate,
    params,
    accessRequestId,
    ...filteringProps,
    ...paginationProps,
  };
}

export type State = ReturnType<typeof useKubes>;
