import { useAppContext } from '../appContextProvider';
import useAsync from '../useAsync';
import { useEffect } from 'react';

export function useClusterLogout({ clusterUri, onClose, clusterTitle }: Props) {
  const ctx = useAppContext();
  const [{ status, statusText }, removeCluster] = useAsync(async () => {
    // TODO(gzdunek): logout and removeCluster should be combined into a single acton in tshd
    await ctx.clustersService.logout(clusterUri);
    await ctx.clustersService.removeCluster(clusterUri);

    if (ctx.workspacesService.getRootClusterUri() === clusterUri) {
      const [firstConnectedWorkspace] =
        ctx.workspacesService.getConnectedWorkspacesClustersUri();
      if (firstConnectedWorkspace) {
        await ctx.workspacesService.setActiveWorkspace(firstConnectedWorkspace);
      } else {
        await ctx.workspacesService.setActiveWorkspace(null);
      }
    }
  });

  useEffect(() => {
    if (status === 'success') {
      onClose();
    }
  }, [status]);

  return {
    status,
    statusText,
    removeCluster,
    onClose,
    clusterUri,
    clusterTitle,
  };
}

export type Props = {
  onClose(): void;
  clusterTitle: string;
  clusterUri: string;
};

export type State = ReturnType<typeof useClusterLogout>;
