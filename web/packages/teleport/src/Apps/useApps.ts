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

import Ctx from 'teleport/teleportContext';
import useStickerClusterId from 'teleport/useStickyClusterId';
import {
  useUrlFiltering,
  useServerSidePagination,
} from 'teleport/components/hooks';

export function useApps(ctx: Ctx) {
  const canCreate = ctx.storeUser.getTokenAccess().create;
  const { isLeafCluster, clusterId } = useStickerClusterId();
  const isEnterprise = ctx.isEnterprise;

  const { params, search, ...filteringProps } = useUrlFiltering({
    sort: {
      fieldName: 'name',
      dir: 'ASC',
    },
  });

  const { fetch, ...paginationProps } = useServerSidePagination({
    fetchFunc: ctx.appService.fetchApps,
    clusterId,
    params,
  });

  useEffect(() => {
    fetch();
  }, [clusterId, search]);

  return {
    clusterId,
    isLeafCluster,
    isEnterprise,
    canCreate,
    params,
    ...filteringProps,
    ...paginationProps,
  };
}

export type State = ReturnType<typeof useApps>;
