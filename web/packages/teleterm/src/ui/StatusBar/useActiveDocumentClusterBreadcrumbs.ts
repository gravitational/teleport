import { useAppContext } from 'teleterm/ui/appContextProvider';
import { getResourceUri } from 'teleterm/ui/services/workspacesService';
import { routing } from 'teleterm/ui/uri';

export function useActiveDocumentClusterBreadcrumbs(): string {
  const ctx = useAppContext();
  ctx.workspacesService.useState();
  ctx.clustersService.useState();

  const activeDocument = ctx.workspacesService
    .getActiveWorkspaceDocumentService()
    ?.getActive();

  if (!activeDocument) {
    return;
  }

  const resourceUri = getResourceUri(activeDocument);
  if (!resourceUri) {
    return;
  }

  const clusterUri = routing.ensureClusterUri(resourceUri);
  const rootClusterUri = routing.ensureRootClusterUri(resourceUri);

  const rootCluster = ctx.clustersService.findCluster(rootClusterUri);
  const leafCluster =
    clusterUri === rootClusterUri
      ? undefined
      : ctx.clustersService.findCluster(clusterUri);

  return [rootCluster, leafCluster]
    .filter(Boolean)
    .map(c => c.name)
    .join(' > ');
}
