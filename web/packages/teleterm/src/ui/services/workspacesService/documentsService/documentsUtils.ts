import { routing } from 'teleterm/ui/uri';

import { Document } from './types';

export function getResourceUri(document: Document): string {
  switch (document.kind) {
    case 'doc.cluster':
      return document.clusterUri;
    case 'doc.gateway':
      return document.targetUri;
    case 'doc.terminal_tsh_node':
      return document.serverUri;
    case 'doc.terminal_tsh_kube':
      return document.kubeUri;
    case 'doc.terminal_shell':
      return routing.getClusterUri(document);
    case 'doc.blank':
      return undefined;
    default:
      assertUnreachable(document);
  }
}

function assertUnreachable(x: never): never {
  throw new Error(`Unhandled case: ${x}`);
}
