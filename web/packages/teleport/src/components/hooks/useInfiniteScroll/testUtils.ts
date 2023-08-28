import { UrlResourcesParams } from 'teleport/config';
import { ApiError } from 'teleport/services/api/parseError';
import { RenderResult } from '@testing-library/react-hooks';
import { State } from './useKeyBasedPagination';

export interface TestResource {
  name: string;
  clusterId: string;
}

function makeTestResources(
  clusterId: string,
  namePrefix: string,
  n: number
): TestResource[] {
  return Array(n)
    .fill(0)
    .map((_, i) => ({ clusterId: clusterId, name: `${namePrefix}${i}` }));
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

export function resourceNames(
  result: RenderResult<{ resources: TestResource[] }>
) {
  return result.current.resources.map((r: any) => r.name);
}

export function resourceClusterIds(
  result: RenderResult<{ resources: TestResource[] }>
) {
  return result.current.resources.map((r: any) => r.clusterId);
}
