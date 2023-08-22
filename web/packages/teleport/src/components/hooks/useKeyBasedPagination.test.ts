/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { renderHook, act, RenderResult } from '@testing-library/react-hooks';

import { UrlResourcesParams } from 'teleport/config';
import { ApiError } from 'teleport/services/api/parseError';

import { useKeyBasedPagination, Props, State } from './useKeyBasedPagination';

interface TestResource {
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

function newDOMAbortError() {
  return new DOMException('Aborted', 'AbortError');
}

function newApiAbortError() {
  return new ApiError('The user aborted a request', new Response(), {
    cause: newDOMAbortError(),
  });
}

/**
 * Creates a mock fetch function that pretends to query a pool of given number
 * of resources. To simulate a search, `params.search` is used as a resource
 * name prefix.
 */
function newFetchFunc(
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
        throw newAbortError();
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

function resourceNames(result: RenderResult<State<TestResource>>) {
  return result.current.resources.map((r: any) => r.name);
}

function resourceClusterIds(result: RenderResult<State<TestResource>>) {
  return result.current.resources.map((r: any) => r.clusterId);
}

function hookProps(overrides: Partial<Props<TestResource>> = {}) {
  return {
    fetchFunc: newFetchFunc(7),
    clusterId: 'test-cluster',
    filter: {},
    initialFetchSize: 2,
    fetchMoreSize: 3,
    ...overrides,
  };
}

test.each`
  n     | names
  ${3}  | ${['r0', 'r1', 'r2']}
  ${4}  | ${['r0', 'r1', 'r2', 'r3']}
  ${10} | ${['r0', 'r1', 'r2', 'r3']}
`('fetches one data batch, n=$n', async ({ n, names }) => {
  const { result } = renderHook(() =>
    useKeyBasedPagination(
      hookProps({
        fetchFunc: newFetchFunc(4),
        initialFetchSize: n,
      })
    )
  );

  expect(result.current.resources).toEqual([]);
  await act(result.current.fetch);
  expect(resourceNames(result)).toEqual(names);
});

test('fetches multiple data batches', async () => {
  const { result } = renderHook(() => useKeyBasedPagination(hookProps()));
  expect(result.current.finished).toBe(false);

  await act(result.current.fetch);
  await act(result.current.fetch);
  expect(resourceNames(result)).toEqual(['r0', 'r1', 'r2', 'r3', 'r4']);
  expect(result.current.finished).toBe(false);
  await act(result.current.fetch);

  const allResources = ['r0', 'r1', 'r2', 'r3', 'r4', 'r5', 'r6'];
  expect(resourceNames(result)).toEqual(allResources);
  expect(result.current.finished).toBe(true);
  await act(result.current.fetch);
  expect(resourceNames(result)).toEqual(allResources);
  expect(result.current.finished).toBe(true);
});

test('maintains attempt state', async () => {
  const { result } = renderHook(() => useKeyBasedPagination(hookProps()));

  expect(result.current.attempt.status).toBe('');
  let fetchPromise;
  act(() => {
    fetchPromise = result.current.fetch();
  });
  expect(result.current.attempt.status).toBe('processing');
  await act(async () => fetchPromise);
  expect(result.current.attempt.status).toBe('success');

  act(() => {
    fetchPromise = result.current.fetch();
  });
  expect(result.current.attempt.status).toBe('processing');
  await act(async () => fetchPromise);
  expect(result.current.attempt.status).toBe('success');
});

test('restarts after query params change', async () => {
  let props = hookProps({
    fetchFunc: newFetchFunc(4),
    clusterId: 'cluster1',
    filter: { search: 'foo' },
  });
  const { result, rerender } = renderHook(useKeyBasedPagination, {
    initialProps: props,
  });

  await act(result.current.fetch);
  expect(resourceClusterIds(result)).toEqual(['cluster1', 'cluster1']);
  expect(resourceNames(result)).toEqual(['foo0', 'foo1']);

  props = { ...props, clusterId: 'cluster2' };
  rerender(props);
  await act(result.current.fetch);
  expect(resourceClusterIds(result)).toEqual(['cluster2', 'cluster2']);

  props = { ...props, filter: { search: 'bar' } };
  rerender(props);
  await act(result.current.fetch);
  expect(resourceNames(result)).toEqual(['bar0', 'bar1']);

  // Make sure we reached the end of the data set.
  await act(result.current.fetch);
  expect(result.current.finished).toBe(true);
  props = { ...props, clusterId: 'cluster3' };
  rerender(props);
  expect(result.current.finished).toBe(false);

  await act(result.current.fetch);
  expect(resourceClusterIds(result)).toEqual(['cluster3', 'cluster3']);
});

test("doesn't restart if params didn't change on rerender", async () => {
  const { result, rerender } = renderHook(useKeyBasedPagination, {
    initialProps: hookProps(),
  });
  await act(result.current.fetch);
  expect(resourceNames(result)).toEqual(['r0', 'r1']);
  rerender(hookProps());
  await act(result.current.fetch);
  expect(resourceNames(result)).toEqual(['r0', 'r1', 'r2', 'r3', 'r4']);
});

describe("doesn't react to fetch() calls before the previous one finishes", () => {
  let props, fetchSpy;

  beforeEach(() => {
    props = hookProps();
    fetchSpy = jest.spyOn(props, 'fetchFunc');
  });

  test('when called once per state reconciliation cycle', async () => {
    const { result } = renderHook(() => useKeyBasedPagination(props));
    let f1, f2;
    act(() => {
      f1 = result.current.fetch();
    });
    act(() => {
      f2 = result.current.fetch();
    });

    await act(async () => f1);
    await act(async () => f2);
    expect(resourceNames(result)).toEqual(['r0', 'r1']);
    expect(props.fetchFunc).toHaveBeenCalledTimes(1);
  });

  test('when called multiple times per state reconciliation cycle', async () => {
    const { result } = renderHook(() => useKeyBasedPagination(props));
    let f1, f2;
    act(() => {
      f1 = result.current.fetch();
      f2 = result.current.fetch();
    });
    await act(async () => f1);
    await act(async () => f2);
    expect(resourceNames(result)).toEqual(['r0', 'r1']);
    expect(fetchSpy).toHaveBeenCalledTimes(1);
  });
});

test.each([
  ['DOMException', newDOMAbortError],
  ['ApiError', newApiAbortError],
])('aborts pending request if params change (%s)', async (_, newError) => {
  let props = hookProps({
    clusterId: 'cluster1',
    fetchFunc: newFetchFunc(7, newError),
  });
  const { result, rerender } = renderHook(useKeyBasedPagination, {
    initialProps: props,
  });
  let fetchPromise;
  act(() => {
    fetchPromise = result.current.fetch();
  });
  props = { ...props, clusterId: 'cluster2' };
  rerender(props);
  await act(async () => fetchPromise);
  expect(resourceClusterIds(result)).toEqual([]);
  await act(result.current.fetch);
  expect(resourceClusterIds(result)).toEqual(['cluster2', 'cluster2']);
});

describe.each`
  name          | ErrorType
  ${'Error'}    | ${Error}
  ${'ApiError'} | ${ApiError}
`('for error type $name', ({ ErrorType }) => {
  it('stops fetching more pages once error is encountered', async () => {
    const props = hookProps();
    const { result } = renderHook(() => useKeyBasedPagination(props));
    const fetchSpy = jest.spyOn(props, 'fetchFunc');

    await act(result.current.fetch);
    expect(resourceNames(result)).toEqual(['r0', 'r1']);

    fetchSpy.mockImplementationOnce(async () => {
      throw new ErrorType('OMGOMG');
    });
    await act(result.current.fetch);
    expect(result.current.attempt.status).toBe('failed');
    expect(resourceNames(result)).toEqual(['r0', 'r1']);

    await act(result.current.fetch);
    expect(result.current.attempt.status).toBe('failed');
    expect(resourceNames(result)).toEqual(['r0', 'r1']);
  });

  it('restarts fetching after error if params change', async () => {
    let props = hookProps({ clusterId: 'cluster1' });
    const fetchSpy = jest.spyOn(props, 'fetchFunc');

    const { result, rerender } = renderHook(useKeyBasedPagination, {
      initialProps: props,
    });
    await act(result.current.fetch);
    expect(resourceClusterIds(result)).toEqual(['cluster1', 'cluster1']);

    fetchSpy.mockImplementationOnce(async () => {
      throw new ErrorType('OMGOMG');
    });

    // Rerender with the same options: still no action expected.
    rerender(props);
    await act(result.current.fetch);
    expect(resourceClusterIds(result)).toEqual(['cluster1', 'cluster1']);

    // Rerender with different props: expect new data to be fetched.
    props = { ...props, clusterId: 'cluster2' };
    rerender(props);
    await act(result.current.fetch);
    expect(resourceClusterIds(result)).toEqual(['cluster2', 'cluster2']);
  });

  it('resumes fetching once forceFetch is called after an error', async () => {
    const props = hookProps();
    const { result } = renderHook(() => useKeyBasedPagination(props));
    const fetchSpy = jest.spyOn(props, 'fetchFunc');

    await act(result.current.fetch);
    fetchSpy.mockImplementationOnce(async () => {
      throw new ErrorType('OMGOMG');
    });
    await act(result.current.fetch);
    await act(result.current.forceFetch);

    expect(result.current.attempt.status).toBe('success');
    expect(resourceNames(result)).toEqual(['r0', 'r1', 'r2', 'r3', 'r4']);
  });
});

test('forceFetch() spawns another request, even if there is one pending', async () => {
  const props = hookProps();
  const fetchSpy = jest.spyOn(props, 'fetchFunc');
  const { result } = renderHook(() => useKeyBasedPagination(props));
  fetchSpy.mockImplementationOnce(async () => {
    return { agents: [{ name: 'impostor', clusterId: 'sus' }] };
  });
  let f1, f2;
  act(() => {
    f1 = result.current.fetch();
  });
  act(() => {
    f2 = result.current.forceFetch();
  });
  await act(async () => Promise.all([f1, f2]));
  expect(resourceNames(result)).toEqual(['r0', 'r1']);
});
