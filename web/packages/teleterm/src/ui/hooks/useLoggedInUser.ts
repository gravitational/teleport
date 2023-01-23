import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { LoggedInUser } from 'teleterm/services/tshd/types';

/**
 * useLoggedInUser returns the logged in user of the root cluster of the currently active workspace.
 * The return value changes depending on the active workspace.
 *
 * It should be used within components that reside outside of WorkspaceContext, typically anything
 * that's outside of Document-type components.
 *
 * It might return undefined if there's no active workspace or during the logout procedure because
 * ClustersService state is cleared up before WorkspacesService state.
 */
export function useLoggedInUser(): LoggedInUser | undefined {
  const { clustersService, workspacesService } = useAppContext();
  clustersService.useState();
  workspacesService.useState();

  const clusterUri = workspacesService.getRootClusterUri();
  if (!clusterUri) {
    return;
  }

  const cluster = clustersService.findCluster(clusterUri);
  return cluster?.loggedInUser;
}

/**
 * useWorkspaceLoggedInUser returns the logged in user of the root cluster of the workspace
 * specified by WorkspaceContext. The returned value won't change when the UI switches between
 * workspaces.
 *
 * It should be used for components which are bound to a particular workspace and which don't change
 * their workspace over their lifecycle; typically those are Document-type components and anything
 * rendered inside of them.
 *
 * It will return undefined during the logout process as ClustersService state is cleared up before
 * WorkspacesService state. There might be other situations in which it returns undefined as well,
 * so for now it's best to always guard against this.
 */
export function useWorkspaceLoggedInUser(): LoggedInUser | undefined {
  const { clustersService } = useAppContext();
  clustersService.useState();
  const { rootClusterUri } = useWorkspaceContext();

  const cluster = clustersService.findCluster(rootClusterUri);
  return cluster?.loggedInUser;
}
