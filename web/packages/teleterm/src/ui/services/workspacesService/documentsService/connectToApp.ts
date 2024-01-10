/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { routing } from 'teleterm/ui/uri';
import { IAppContext } from 'teleterm/ui/types';

import { App } from 'teleterm/services/tshd/types';

import { DocumentOrigin } from './types';

export async function connectToApp(
  ctx: IAppContext,
  target: App,
  telemetry: { origin: DocumentOrigin }
): Promise<void> {
  //TODO(gzdunek): Add regular dialogs for connecting to unsupported apps (non HTTP/TCP)
  // that will explain that the user can connect via tsh/Web UI to them.
  // These dialogs should provide instructions, just like those in the Web UI for database access.
  if (target.samlApp) {
    alert('SAML apps are supported only in Web UI.');
    return;
  }

  if (target.awsConsole) {
    alert('AWS apps are supported in Web UI and tsh.');
    return;
  }

  if (target.endpointUri.startsWith('cloud://')) {
    alert('Cloud apps are supported only in tsh.');
    return;
  }

  const rootClusterUri = routing.ensureRootClusterUri(target.uri);
  const documentsService =
    ctx.workspacesService.getWorkspaceDocumentService(rootClusterUri);
  const doc = documentsService.createGatewayDocument({
    targetUri: target.uri,
    origin: telemetry.origin,
    targetName: routing.parseAppUri(target.uri).params.appId,
    targetUser: '',
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
