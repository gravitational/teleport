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

import fs from 'node:fs/promises';

import { BrowserWindow } from 'electron';
import { Cluster } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';

import { RendererIpc } from 'teleterm/mainProcess/types';
import { TshdClient } from 'teleterm/services/tshd';
import { RootClusterUri } from 'teleterm/ui/uri';

export async function startProfileWatcher(
  tshd: TshdClient,
  window: BrowserWindow,
  path: string
): Promise<void> {
  const {
    response: { clusters: initialClusters },
  } = await tshd.listRootClusters({});
  const oldClusters = new Map(initialClusters.map(c => [c.uri, c]));

  const watcher = fs.watch(path);
  try {
    for await (const event of watcher) {
      if (event.eventType === 'rename') {
        const {
          response: { clusters },
        } = await tshd.listRootClusters({});
        const newClusters = new Map(clusters.map(c => [c.uri, c]));
        const changes = detectChanges(oldClusters, newClusters);

        window.webContents.send(RendererIpc.ProfileChange, changes);
      }
    }
  } catch (e) {
    if (e.name === 'AbortError') {
      return;
    }
    throw e;
  }
}

type Clusters = Map<string, Cluster>;

export type ProfileChange = {
  op: 'added' | 'removed';
  uri: RootClusterUri;
};

function detectChanges(
  oldClusters: Clusters,
  newClusters: Clusters
): ProfileChange[] {
  const changes: ProfileChange[] = [];

  for (const [uri] of oldClusters) {
    if (!newClusters.has(uri)) {
      changes.push({ op: 'removed', uri });
    }
  }

  for (const [uri] of newClusters) {
    if (!oldClusters.has(uri)) {
      changes.push({ op: 'added', uri });
    }
  }

  return changes;
}
