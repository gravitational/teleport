import { useAppContext } from 'teleterm/ui/appContextProvider';
import { Cluster } from 'teleterm/services/tshd/types';

export function useIdentity() {
  const ctx = useAppContext();

  ctx.clustersService.useState();
  ctx.workspacesService.useState();

  function changeRootCluster(clusterUri: string): Promise<void> {
    return ctx.workspacesService.setActiveWorkspace(clusterUri);
  }

  function addCluster(): void {
    ctx.commandLauncher.executeCommand('cluster-connect', {});
  }

  function removeCluster(clusterUri: string): void {
    ctx.commandLauncher.executeCommand('cluster-remove', { clusterUri });
  }

  async function logout(): Promise<void> {
    const rootClusterUri = ctx.workspacesService.getRootClusterUri();
    await ctx.clustersService.logout(rootClusterUri);
    const [firstConnectedWorkspace] =
      ctx.workspacesService.getConnectedWorkspacesClustersUri();
    if (firstConnectedWorkspace) {
      await ctx.workspacesService.setActiveWorkspace(firstConnectedWorkspace);
    } else {
      await ctx.workspacesService.setActiveWorkspace(null);
    }
  }

  function getActiveRootCluster(): Cluster | undefined {
    const clusterUri = ctx.workspacesService.getRootClusterUri();
    if (!clusterUri) {
      return;
    }
    return ctx.clustersService.findCluster(clusterUri);
  }

  const rootClusters: IdentityRootCluster[] = ctx.clustersService
    .getClusters()
    .filter(c => !c.leaf)
    .map(cluster => ({
      active: cluster.uri === ctx.workspacesService.getRootClusterUri(),
      clusterName: cluster.name,
      userName: cluster.loggedInUser?.name,
      uri: cluster.uri,
      connected: cluster.connected,
      clusterSyncStatus: ctx.clustersService.getClusterSyncStatus(cluster.uri)
        .syncing,
    }));

  return {
    changeRootCluster,
    addCluster,
    removeCluster,
    logout,
    activeRootCluster: getActiveRootCluster(),
    rootClusters,
  };
}

export interface IdentityRootCluster {
  active: boolean;
  clusterName: string;
  userName: string;
  uri: string;
  connected: boolean;
  clusterSyncStatus: boolean;
}
