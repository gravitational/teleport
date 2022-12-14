import { ClusterOrResourceUri, routing } from 'teleterm/ui/uri';
import { assertUnreachable } from 'teleterm/ui/utils';

import { Document } from './types';

export function getResourceUri(document: Document): ClusterOrResourceUri {
  switch (document.kind) {
    case 'doc.cluster':
      return document.clusterUri;
    case 'doc.gateway':
      return document.targetUri;
    case 'doc.terminal_tsh_node':
      return document.serverUri;
    case 'doc.terminal_tsh_kube':
      return document.kubeUri;
    case 'doc.access_requests':
      return document.clusterUri;
    case 'doc.terminal_shell':
      return routing.getClusterUri({
        rootClusterId: document.rootClusterId,
        leafClusterId: document.leafClusterId,
      });
    case 'doc.blank':
      return undefined;
    default:
      assertUnreachable(document);
  }
}
