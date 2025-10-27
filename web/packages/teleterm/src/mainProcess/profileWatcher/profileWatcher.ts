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

import { watch } from 'node:fs';

import { Cluster } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';
import { debounce } from 'shared/utils/highbar';

import { makeClusterWithOnlyProfileProperties } from 'teleterm/services/tshd/cluster';
import { RootClusterUri } from 'teleterm/ui/uri';

interface TshClient {
  listRootClusters(): Promise<Cluster[]>;
}

interface ClusterStore {
  getRootClusters(): Cluster[];
}

/**
 * Watches the specified `tshDirectory` for profile changes.
 * File system events are debounced with a 200 ms delay.
 */
export async function* watchProfiles(
  tshDirectory: string,
  tshClient: TshClient,
  clusterStore: ClusterStore,
  options?: { signal?: AbortSignal }
): AsyncGenerator<ProfileChangeSet, void, void> {
  // eslint-disable-next-line unused-imports/no-unused-vars
  for await (const _ of debounceWatch(tshDirectory, 200, options?.signal)) {
    const clusters = await tshClient.listRootClusters();
    const newClusters = new Map(clusters.map(c => [c.uri, c]));
    const oldClusters = new Map(
      clusterStore.getRootClusters().map(c => [c.uri, c])
    );

    const changes = detectChanges(oldClusters, newClusters);
    if (changes.length > 0) {
      yield changes;
    }
  }
}

export type ProfileChange =
  | {
      /** A cluster has been added. */
      op: 'added';
      cluster: Cluster;
    }
  | {
      /** A cluster has been removed. */
      op: 'removed';
      cluster: Cluster;
    }
  | {
      /**
       * A cluster's properties have changed.
       * Only the properties present locally in the profile are compared.
       * (`listRootClusters` doesn't return cluster details from the proxy).
       */
      op: 'changed';
      previous: Cluster;
      next: Cluster;
    };

export type ProfileChangeSet = ProfileChange[];

async function* debounceWatch(
  path: string,
  debounceMs: number,
  abortSignal: AbortSignal | undefined
): AsyncGenerator<void> {
  let signal = Promise.withResolvers<void>();
  let closed = false;
  const scheduleYield = debounce(() => signal.resolve(), debounceMs);

  const watcher = watch(
    path,
    { signal: abortSignal, recursive: true },
    scheduleYield
  );

  const closeHandler = () => {
    closed = true;
    signal.resolve();
  };
  const errorHandler = (e: Error) => signal.reject(e);
  watcher.on('close', closeHandler);
  watcher.on('error', errorHandler);

  try {
    while (true) {
      await signal.promise;
      if (closed) {
        break;
      }

      // Recreate the signal so any events occurring while yielding will resolve the next promise.
      signal = Promise.withResolvers();

      yield;
    }
  } finally {
    scheduleYield.cancel();
    // Unblocks the loop.
    watcher.close();
    watcher.off('close', closeHandler);
    watcher.off('error', errorHandler);
  }
}

function detectChanges(
  previousClusters: Map<RootClusterUri, Cluster>,
  nextClusters: Map<RootClusterUri, Cluster>
): ProfileChange[] {
  const changes: ProfileChange[] = [];
  const allUris = new Set([...previousClusters.keys(), ...nextClusters.keys()]);

  for (const uri of allUris) {
    const previous = previousClusters.get(uri);
    const next = nextClusters.get(uri);

    if (!nextClusters.has(uri)) {
      changes.push({
        op: 'removed',
        cluster: previous,
      });
    } else if (!previousClusters.has(uri)) {
      changes.push({ op: 'added', cluster: next });
    } else if (
      !Cluster.equals(
        makeClusterWithOnlyProfileProperties(previous),
        makeClusterWithOnlyProfileProperties(next)
      )
    ) {
      changes.push({
        op: 'changed',
        previous: previous,
        next: next,
      });
    }
  }

  return changes;
}
