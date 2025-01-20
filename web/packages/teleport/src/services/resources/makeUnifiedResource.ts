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

import makeGitServer from 'teleport/services/gitServers/makeGitServer';

import { UnifiedResource, UnifiedResourceKind } from '../agents';
import makeApp from '../apps/makeApps';
import { makeDatabase } from '../databases/makeDatabase';
import { makeDesktop } from '../desktops/makeDesktop';
import { makeKube } from '../kube/makeKube';
import makeNode from '../nodes/makeNode';

export function makeUnifiedResource(json: any): UnifiedResource {
  json = json || {};

  switch (json.kind as UnifiedResourceKind) {
    case 'app':
      return makeApp(json);
    case 'db':
      return makeDatabase(json);
    case 'kube_cluster':
      return makeKube(json);
    case 'node':
      return makeNode(json);
    case 'windows_desktop':
      return makeDesktop(json);
    case 'git_server':
      return makeGitServer(json);
    default:
      throw new Error(`Unknown unified resource kind: "${json.kind}"`);
  }
}
