import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useCallback } from 'react';

export function useNewTabOpener() {
  const ctx = useAppContext();

  const documentsService =
    ctx.workspacesService.getActiveWorkspaceDocumentService();

  const localClusterUri =
    ctx.workspacesService.getActiveWorkspace()?.localClusterUri;

  const openClusterTab = useCallback(() => {
    if (localClusterUri) {
      const clusterDocument = documentsService.createClusterDocument({
        clusterUri: localClusterUri,
      });

      documentsService.add(clusterDocument);
      documentsService.open(clusterDocument.uri);
    }
  }, [documentsService, localClusterUri]);

  return { openClusterTab };
}
