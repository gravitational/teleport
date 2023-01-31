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
