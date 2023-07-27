import { AgentKind, UnifiedResourceKind } from '../agents';
import makeApp from '../apps/makeApps';
import { makeDatabase } from '../databases/makeDatabase';
import { makeDesktop } from '../desktops/makeDesktop';
import makeNode from '../nodes/makeNode';

export function makeUnifiedResource(json: any): AgentKind {
  json = json || {};

  switch (json.kind as UnifiedResourceKind) {
    case 'app':
      return makeApp(json);
    case 'db':
      return makeDatabase(json);
    case 'node':
      return makeNode(json);
    case 'windows_desktop':
      return makeDesktop(json);
    default:
      throw new Error(`Unknown unified resource kind: "${json.kind}"`);
  }
}
