import { useCallback } from 'react';

import { DocumentsService } from 'teleterm/ui/services/workspacesService';
import { ClusterUri } from 'teleterm/ui/uri';

export function useNewTabOpener({
  documentsService,
  localClusterUri,
}: {
  documentsService: DocumentsService;
  localClusterUri: ClusterUri;
}) {
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
