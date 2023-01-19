import { useAppContext } from 'teleterm/ui/appContextProvider';

export function useLoggedInUser() {
  const ctx = useAppContext();

  ctx.clustersService.useState();
  ctx.workspacesService.useState();

  const clusterUri = ctx.workspacesService.getRootClusterUri();
  if (!clusterUri) {
    return;
  }

  const cluster = ctx.clustersService.findCluster(clusterUri);
  if (!cluster) {
    return;
  }

  return cluster.loggedInUser;
}
