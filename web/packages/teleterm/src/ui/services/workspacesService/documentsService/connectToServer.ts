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

import { IAppContext } from 'teleterm/ui/types';
import { routing, ServerUri } from 'teleterm/ui/uri';

import { DocumentOrigin } from './types';

export async function connectToServer(
  ctx: IAppContext,
  target: {
    uri: ServerUri;
    hostname: string;
    login: string;
  },
  telemetry: {
    origin: DocumentOrigin;
  }
): Promise<void> {
  const rootClusterUri = routing.ensureRootClusterUri(target.uri);
  const documentsService = ctx.workspacesService.getWorkspaceDocumentService(
    routing.ensureRootClusterUri(target.uri)
  );
  const doc = documentsService.createTshNodeDocument(target.uri, {
    origin: telemetry.origin,
  });
  doc.title = `${target.login}@${target.hostname}`;
  doc.login = target.login;

  await ctx.workspacesService.setActiveWorkspace(rootClusterUri);
  documentsService.add(doc);
  documentsService.open(doc.uri);
}
