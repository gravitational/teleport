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

import 'whatwg-fetch';

// eslint-disable-next-line no-restricted-imports -- FIXME
import { Node } from 'teleport/services/nodes';

/**
 * Creates `n` nodes. We use the `Node` type for testing, because it's slim and
 * it has a `clusterId` field.
 */
function makeTestResources(
  clusterId: string,
  namePrefix: string,
  n: number
): Node[] {
  return Array(n)
    .fill(0)
    .map((_, i) => ({
      kind: 'node',
      subKind: 'teleport',
      id: i.toString(),
      clusterId: clusterId,
      hostname: `${namePrefix}${i}`,
      labels: [],
      addr: '',
      tunnel: false,
      sshLogins: [],
    }));
}

export function newDOMAbortError() {
  return new DOMException('Aborted', 'AbortError');
}

/**
 * Creates a mock fetch function that pretends to query a pool of given number
 * of resources. To simulate a search, `params.search` is used as a resource
 * name prefix.
 */
export function newFetchFunc({
  clusterId = 'test-cluster',
  search,
  numResources,
  newAbortError = newDOMAbortError,
}: {
  clusterId?: string;
  search?: string;
  numResources: number;
  newAbortError?: () => Error;
}) {
  return async (
    params: {
      limit: number;
      startKey: string;
    },
    signal?: AbortSignal
  ) => {
    const { startKey, limit } = params;
    const startIndex = parseInt(startKey || '0');
    const namePrefix = search ?? 'r';
    const endIndex = startIndex + limit;
    const nextStartKey =
      endIndex < numResources ? endIndex.toString() : undefined;
    if (signal) {
      // Give the caller a chance to abort the request.
      await Promise.resolve();
      if (signal.aborted) {
        const err = newAbortError();
        if (err) throw err;
      }
    }
    return {
      agents: makeTestResources(clusterId, namePrefix, numResources).slice(
        startIndex,
        startIndex + limit
      ),
      startKey: nextStartKey,
    };
  };
}

export function resourceNames(result: { resources: Node[] }) {
  return result.resources.map(r => r.hostname);
}

export function resourceClusterIds(result: { resources: Node[] }) {
  return result.resources.map(r => r.clusterId);
}
