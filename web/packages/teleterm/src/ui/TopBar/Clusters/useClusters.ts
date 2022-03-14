import { useAppContext } from 'teleterm/ui/appContextProvider';

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

  const rootClusterUri = workspacesService.getRootClusterUri();
  const localClusterUri =
    workspacesService.getActiveWorkspace()?.localClusterUri;

  return {
    selectedItem:
      localClusterUri && clustersService.findCluster(localClusterUri),
    selectItem: (localClusterUri: string) => {
      workspacesService.setWorkspaceLocalClusterUri(
        rootClusterUri,
        localClusterUri
      );
      commandLauncher.executeCommand('cluster-open', {
        clusterUri: localClusterUri,
      });
    },
    items: rootClusterUri
      ? [
          clustersService.findCluster(rootClusterUri),
          ...findLeaves(rootClusterUri),
        ]
      : [],
  };
}
