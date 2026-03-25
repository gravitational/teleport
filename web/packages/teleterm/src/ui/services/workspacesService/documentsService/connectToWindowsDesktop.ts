/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { createDesktopSessionDocument } from 'teleterm/ui/services/workspacesService';
import { IAppContext } from 'teleterm/ui/types';
import { routing, WindowsDesktopUri } from 'teleterm/ui/uri';

import { DocumentOrigin } from './types';

export async function connectToWindowsDesktop(
  ctx: IAppContext,
  target: {
    uri: WindowsDesktopUri;
    login: string;
  },
  telemetry: {
    origin: DocumentOrigin;
  }
): Promise<void> {
  const rootClusterUri = routing.ensureRootClusterUri(target.uri);
  await ctx.workspacesService.setActiveWorkspace(rootClusterUri);
  ctx.workspacesService
    .getWorkspaceDocumentService(rootClusterUri)
    .openExistingOrAddNew(
      doc => {
        return (
          doc.kind === 'doc.desktop_session' &&
          doc.desktopUri === target.uri &&
          doc.login === target.login
        );
      },
      () => {
        return createDesktopSessionDocument({
          desktopUri: target.uri,
          login: target.login,
          origin: telemetry.origin,
        });
      }
    );
}
