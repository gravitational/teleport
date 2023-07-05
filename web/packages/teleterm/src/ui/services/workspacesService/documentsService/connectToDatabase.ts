/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { IAppContext } from 'teleterm/ui/types';
import { DatabaseUri, routing } from 'teleterm/ui/uri';
import { GatewayProtocol } from 'teleterm/services/tshd/types';

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
