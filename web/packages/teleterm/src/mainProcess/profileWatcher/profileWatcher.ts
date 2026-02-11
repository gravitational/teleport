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
import { access } from 'node:fs/promises';

import { Cluster } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';
import { debounce } from 'shared/utils/highbar';
import { wait } from 'shared/utils/wait';

import { isTshdRpcError } from 'teleterm/services/tshd';
import { mergeClusterProfileWithDetails } from 'teleterm/services/tshd/cluster';
import { RootClusterUri } from 'teleterm/ui/uri';

interface TshClient {
  listRootClusters(): Promise<Cluster[]>;
}

interface ClusterStore {
  getRootClusters(): Cluster[];
}

/**
 * Watches the specified `tshDirectory` for profile changes.
 * File system events are debounced with a default 200 ms delay.
 * The watcher should be started only after the initial cluster store sync completes,
 * to prevent unnecessary profile change events.
 *
 * When the watched directory is removed, the watcher emits a profile change
 * and enters polling mode (with 1 second interval) until the directory reappears.
 */
export async function* watchProfiles({
  tshDirectory,
  tshClient,
  clusterStore,
  debounceMs = 200,
  maxFileSystemEvents = 4096,
  signal,
}: {
  tshDirectory: string;
  tshClient: TshClient;
  clusterStore: ClusterStore;
  debounceMs?: number;
  /**
   * Maximum number of file system events that can accumulate while debouncing.
   *
   * Note: On Windows, removing a watched directory can trigger an infinite stream
   * of events. Setting this limit helps mitigate that issue.
   *
   * Default 4096.
   * */
  maxFileSystemEvents?: number;
  signal?: AbortSignal;
}): AsyncGenerator<ProfileChangeSet, void, void> {
  while (!signal?.aborted) {
    try {
      // eslint-disable-next-line unused-imports/no-unused-vars
      for await (const _ of debounceWatch(
        tshDirectory,
        debounceMs,
        maxFileSystemEvents,
        signal
      )) {
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
    } catch (error) {
      // Check if the error is caused by removing the watched directory.
      // Removing that directory emits different events, depending on a platform:
      // - On macOS/Linux, it emits a 'rename' event.
      // - On Windows, it may throw an EPERM error, or emit thousands of events
      // (so that we check FileSystemEventsOverflowError).
      // To reliably detect removal on macOS/Linux, we expect tshClient.listRootClusters()
      // to fail with a filesystem-related error, allowing us to catch all relevant cases here.
      if (
        isTshdRpcError(error, 'NOT_FOUND') ||
        error instanceof FileSystemEventsOverflowError ||
        error?.code === 'EPERM'
      ) {
        const ok = await pathExists(tshDirectory);
        if (!ok) {
          yield clusterStore
            .getRootClusters()
            .map(cluster => ({ op: 'removed', cluster }));
          await waitForPath(tshDirectory, signal);
          continue;
        }
      }
      throw error;
    }
  }
}

class FileSystemEventsOverflowError extends Error {
  constructor(maxCount: number, debounceMs: number) {
    super(
      `Exceeded file system event limit: more than ${maxCount} events detected within ${debounceMs} ms`
    );
  }
}

async function pathExists(dirPath: string): Promise<boolean> {
  try {
    await access(dirPath);
    return true;
  } catch (error) {
    if (error.code === 'ENOENT') {
      return false;
    }
    throw error;
  }
}

/** Waits for path to exists, polling at intervals (1 second). */
async function waitForPath(
  dirPath: string,
  signal?: AbortSignal
): Promise<void> {
  if (signal?.aborted) {
    return;
  }

  while (!signal?.aborted) {
    // Start from waiting, pathExists() was invoked earlier to check the path.
    await wait(1000, signal);
    const exist = await pathExists(dirPath);
    if (exist) {
      return;
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
  maxFileSystemEvents: number,
  abortSignal: AbortSignal | undefined
): AsyncGenerator<void> {
  let signal = Promise.withResolvers<void>();
  let closed = false;
  let eventsToDebounce = 0;
  const scheduleYield = debounce(() => {
    eventsToDebounce = 0;
    signal.resolve();
  }, debounceMs);
  const onEvent = () => {
    ++eventsToDebounce;
    if (eventsToDebounce > maxFileSystemEvents) {
      signal.reject(
        new FileSystemEventsOverflowError(maxFileSystemEvents, debounceMs)
      );
      return;
    }
    scheduleYield();
  };

  const watcher = watch(
    path,
    { signal: abortSignal, recursive: true },
    onEvent
  );

  const closeHandler = () => {
    closed = true;
    signal.resolve();
  };
  const errorHandler = (e: Error) => signal.reject(e);
  watcher.on('close', closeHandler);
  watcher.on('error', errorHandler);

  // The watcher might be restarted if the path disappears and then reappears.
  // Begin by checking for any changes immediately.
  onEvent();

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
      // Ensure we are comparing only profile properties.
      !Cluster.equals(
        mergeClusterProfileWithDetails({
          profile: previous,
          details: Cluster.create(),
        }),
        mergeClusterProfileWithDetails({
          profile: next,
          details: Cluster.create(),
        })
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
