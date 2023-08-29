import 'whatwg-fetch';
import { RenderResult } from '@testing-library/react-hooks';

import { UrlResourcesParams } from 'teleport/config';
import { ApiError } from 'teleport/services/api/parseError';

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

export function newApiAbortError() {
  return new ApiError('The user aborted a request', new Response(), {
    cause: newDOMAbortError(),
  });
}

/**
 * Creates a mock fetch function that pretends to query a pool of given number
 * of resources. To simulate a search, `params.search` is used as a resource
 * name prefix.
 */
export function newFetchFunc(
  numResources: number,
  newAbortError: () => Error = newDOMAbortError
) {
  return async (
    clusterId: string,
    params: UrlResourcesParams,
    signal?: AbortSignal
  ) => {
    const { startKey, limit } = params;
    const startIndex = parseInt(startKey || '0');
    const namePrefix = params.search ?? 'r';
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

export function resourceNames(result: RenderResult<{ resources: Node[] }>) {
  return result.current.resources.map(r => r.hostname);
}

export function resourceClusterIds(
  result: RenderResult<{ resources: Node[] }>
) {
  return result.current.resources.map(r => r.clusterId);
}
