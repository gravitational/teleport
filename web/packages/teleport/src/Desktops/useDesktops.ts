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
    fieldName: 'name',
    dir: 'ASC',
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
