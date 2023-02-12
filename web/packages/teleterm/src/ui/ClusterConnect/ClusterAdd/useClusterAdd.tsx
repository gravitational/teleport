/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useAsync } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';

import { ClusterAddProps, ClusterAddPresentationProps } from './ClusterAdd';

export function useClusterAdd(
  props: ClusterAddProps
): ClusterAddPresentationProps {
  const ctx = useAppContext();
  const [{ status, statusText }, addCluster] = useAsync(
    async (addr: string) => {
      const proxyAddr = parseClusterProxyWebAddr(addr);
      const cluster = await ctx.clustersService.addRootCluster(proxyAddr);
      return props.onSuccess(cluster.uri);
    }
  );

  return {
    addCluster,
    status,
    statusText,
    onCancel: props.onCancel,
  };
}

function parseClusterProxyWebAddr(addr: string) {
  addr = addr || '';
  if (addr.startsWith('http')) {
    const url = new URL(addr);
    return url.host;
  }

  return addr;
}
