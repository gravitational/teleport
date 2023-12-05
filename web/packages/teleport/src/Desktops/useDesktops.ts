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

import { LoginItem } from 'shared/components/MenuLogin';

import Ctx from 'teleport/teleportContext';
import cfg from 'teleport/config';
import useStickyClusterId from 'teleport/useStickyClusterId';
import {
  useUrlFiltering,
  useServerSidePagination,
} from 'teleport/components/hooks';
import { openNewTab } from 'teleport/lib/util';

import type { Desktop } from 'teleport/services/desktops';

export function useDesktops(ctx: Ctx) {
  const { clusterId, isLeafCluster } = useStickyClusterId();
  const canCreate = ctx.storeUser.getTokenAccess().create;
  const username = ctx.storeUser.state.username;

  const { params, search, ...filteringProps } = useUrlFiltering({
    sort: {
      fieldName: 'name',
      dir: 'ASC',
    },
  });

  const { fetch, ...paginationProps } = useServerSidePagination({
    fetchFunc: ctx.desktopService.fetchDesktops,
    clusterId,
    params,
  });

  useEffect(() => {
    fetch();
  }, [clusterId, search]);

  const getWindowsLoginOptions = ({ name, logins }: Desktop) =>
    makeOptions(clusterId, name, logins);

  const openRemoteDesktopTab = (username: string, desktopName: string) => {
    const url = cfg.getDesktopRoute({
      clusterId,
      desktopName,
      username,
    });

    openNewTab(url);
  };

  return {
    username,
    clusterId,
    canCreate,
    isLeafCluster,
    getWindowsLoginOptions,
    openRemoteDesktopTab,
    params,
    ...filteringProps,
    ...paginationProps,
  };
}

function makeOptions(
  clusterId: string,
  desktopName = '',
  logins = [] as string[]
): LoginItem[] {
  return logins.map(username => {
    const url = cfg.getDesktopRoute({
      clusterId,
      desktopName,
      username,
    });

    return {
      login: username,
      url,
    };
  });
}

export type State = ReturnType<typeof useDesktops>;
