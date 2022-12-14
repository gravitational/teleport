import { useAppContext } from 'teleterm/ui/appContextProvider';
import { ClusterUri } from 'teleterm/ui/uri';

export function useClusters() {
  const { workspacesService, clustersService, commandLauncher } =
    useAppContext();

  workspacesService.useState();
  clustersService.useState();

  function findLeaves(clusterUri: string) {
    return clustersService
      .getClusters()
      .filter(c => c.leaf && c.uri.startsWith(clusterUri));
  }

  function hasPendingAccessRequest() {
    const accessRequestsService =
      workspacesService.getActiveWorkspaceAccessRequestsService();
    if (!accessRequestsService) {
      return false;
    }

    const pendingAccessRequest =
      accessRequestsService.getPendingAccessRequest();

    if (!pendingAccessRequest) {
      return false;
    }

    const count = accessRequestsService.getAddedResourceCount();
    return count > 0;
  }

  function clearPendingAccessRequest() {
    const accessRequestsService =
      workspacesService.getActiveWorkspaceAccessRequestsService();

    accessRequestsService?.clearPendingAccessRequest();
  }

  const rootClusterUri = workspacesService.getRootClusterUri();
  const localClusterUri =
    workspacesService.getActiveWorkspace()?.localClusterUri;
  const rootCluster = clustersService.findCluster(rootClusterUri);
  const items =
    (rootCluster && [rootCluster, ...findLeaves(rootClusterUri)]) || [];

  return {
    hasLeaves: items.some(i => i.leaf),
    hasPendingAccessRequest: hasPendingAccessRequest(),
    clearPendingAccessRequest,
    selectedItem:
      localClusterUri && clustersService.findCluster(localClusterUri),
    selectItem: (localClusterUri: ClusterUri) => {
      workspacesService.setWorkspaceLocalClusterUri(
        rootClusterUri,
        localClusterUri
      );
      commandLauncher.executeCommand('cluster-open', {
        clusterUri: localClusterUri,
      });
    },
    items,
  };
}
