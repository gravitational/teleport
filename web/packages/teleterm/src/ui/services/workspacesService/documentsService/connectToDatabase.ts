/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { GatewayProtocol } from 'teleterm/services/tshd/types';
import { IAppContext } from 'teleterm/ui/types';
import { DatabaseUri, routing } from 'teleterm/ui/uri';

import { DocumentOrigin } from './types';

export async function connectToDatabase(
  ctx: IAppContext,
  target: {
    uri: DatabaseUri;
    name: string;
    protocol: string;
    dbUser: string;
  },
  telemetry: {
    origin: DocumentOrigin;
  }
): Promise<void> {
  const rootClusterUri = routing.ensureRootClusterUri(target.uri);
  const documentsService =
    ctx.workspacesService.getWorkspaceDocumentService(rootClusterUri);

  const doc = documentsService.createGatewayDocument({
    // Not passing the `gatewayUri` field here, as at this point the gateway doesn't exist yet.
    // `port` is not passed as well, we'll let the tsh daemon pick a random one.
    targetUri: target.uri,
    targetName: target.name,
    targetUser: getTargetUser(
      target.protocol as GatewayProtocol,
      target.dbUser
    ),
    origin: telemetry.origin,
  });

  const connectionToReuse = ctx.connectionTracker.findConnectionByDocument(doc);

  if (connectionToReuse) {
    await ctx.connectionTracker.activateItem(connectionToReuse.id, {
      origin: telemetry.origin,
    });
  } else {
    await ctx.workspacesService.setActiveWorkspace(rootClusterUri);
    documentsService.add(doc);
    documentsService.open(doc.uri);
  }
}

function getTargetUser(
  protocol: GatewayProtocol,
  providedDbUser: string
): string {
  // we are replicating tsh behavior (user can be omitted for Redis)
  // https://github.com/gravitational/teleport/blob/796e37bdbc1cb6e0a93b07115ffefa0e6922c529/tool/tsh/db.go#L240-L244
  // but unlike tsh, Connect has to provide a user that is then used in a gateway document
  if (protocol === 'redis') {
    return providedDbUser || 'default';
  }

  return providedDbUser;
}
