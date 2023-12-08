/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
    sort: {
      fieldName: 'name',
      dir: 'ASC',
    },
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
