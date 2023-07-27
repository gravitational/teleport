/*
Copyright 2019-2022 Gravitational, Inc.

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

import Ctx from 'teleport/teleportContext';
import useStickyClusterId from 'teleport/useStickyClusterId';
import { useUrlFiltering } from 'teleport/components/hooks';
import { useInfiniteScroll } from 'teleport/components/hooks/useInfiniteScroll';

export function useResources(ctx: Ctx) {
  const { clusterId } = useStickyClusterId();
  // const canCreate = ctx.storeUser.getTokenAccess().create;

  const { params, search, ...filteringProps } = useUrlFiltering({
    fieldName: 'name',
    dir: 'ASC',
  });

  const { fetch, fetchedData, attempt, fetchMore } = useInfiniteScroll({
    fetchFunc: ctx.resourceService.fetchResources,
    clusterId,
    params,
  });

  useEffect(() => {
    fetch();
  }, [clusterId, search]);

  return {
    ...filteringProps,
    fetchedData,
    clusterId,
    params,
    fetchMore,
    attempt,
  };
}

// function makeOptions(clusterId: string, node: Node | undefined) {
//   const nodeLogins = node?.sshLogins || [];
//   const logins = sortLogins(nodeLogins);

//   return logins.map(login => {
//     const url = cfg.getSshConnectRoute({
//       clusterId,
//       serverId: node?.id || '',
//       login,
//     });

//     return {
//       login,
//       url,
//     };
//   });
// }

// sort logins by making 'root' as the first in the list
export const sortLogins = (logins: string[]) => {
  const noRoot = logins.filter(l => l !== 'root').sort();
  if (noRoot.length === logins.length) {
    return logins;
  }
  return ['root', ...noRoot];
};

export type State = ReturnType<typeof useResources>;
