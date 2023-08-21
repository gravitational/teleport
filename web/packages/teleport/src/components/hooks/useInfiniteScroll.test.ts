import React from 'react';
import { renderHook, act, RenderResult } from '@testing-library/react-hooks';
import { useKeyBasedPagination, Props, State } from './useInfiniteScroll';
import { UrlResourcesParams } from 'teleport/config';

interface TestResource {
  name: string;
  clusterId: string;
}

function getTestResources(
  clusterId: string,
  namePrefix: string,
  n: number
): TestResource[] {
  return Array(n)
    .fill(0)
    .map((_, i) => ({ clusterId: clusterId, name: `${namePrefix}${i}` }));
}

function newFetchFunc(numResources: number) {
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
        throw new DOMException('Aborted', 'AbortError');
      }
    }
    return {
      agents: getTestResources(clusterId, namePrefix, numResources).slice(
        startIndex,
        startIndex + limit
      ),
      startKey: nextStartKey,
    };
  };
}

function resourceNames(result: RenderResult<State<TestResource>>) {
  return result.current.fetchedData.agents.map((r: any) => r.name);
}

function resourceClusterIds(result: RenderResult<State<TestResource>>) {
  return result.current.fetchedData.agents.map((r: any) => r.clusterId);
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

describe('useKeyBasedPagination', () => {
  it.each`
    n     | names
    ${3}  | ${['r0', 'r1', 'r2']}
    ${4}  | ${['r0', 'r1', 'r2', 'r3']}
    ${10} | ${['r0', 'r1', 'r2', 'r3']}
  `('Fetches one data batch, n=$n', async ({ n, names }) => {
    const { result } = renderHook(() =>
      useKeyBasedPagination(
        hookProps({
          fetchFunc: newFetchFunc(4),
          initialFetchSize: n,
        })
      )
    );

    expect(result.current.fetchedData.agents).toEqual([]);
    await act(async () => result.current.fetch());
    expect(resourceNames(result)).toEqual(names);
  });

  it('fetches multiple data batches', async () => {
    const { result } = renderHook(() => useKeyBasedPagination(hookProps()));

    await act(async () => result.current.fetch());
    await act(async () => result.current.fetch());
    expect(resourceNames(result)).toEqual(['r0', 'r1', 'r2', 'r3', 'r4']);
    await act(async () => result.current.fetch());

    const allResources = ['r0', 'r1', 'r2', 'r3', 'r4', 'r5', 'r6'];
    expect(resourceNames(result)).toEqual(allResources);
    await act(async () => result.current.fetch());
    expect(resourceNames(result)).toEqual(allResources);
  });

  it('maintains attempt state', async () => {
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

  it('restarts after query params change', async () => {
    let props = hookProps({
      fetchFunc: newFetchFunc(4),
      clusterId: 'cluster1',
      filter: { search: 'foo' },
    });
    const { result, rerender } = renderHook(useKeyBasedPagination, {
      initialProps: props,
    });

    await act(async () => result.current.fetch());
    expect(resourceClusterIds(result)).toEqual(['cluster1', 'cluster1']);
    expect(resourceNames(result)).toEqual(['foo0', 'foo1']);

    props = { ...props, clusterId: 'cluster2' };
    rerender(props);
    await act(async () => result.current.fetch());
    expect(resourceClusterIds(result)).toEqual(['cluster2', 'cluster2']);

    props = { ...props, filter: { search: 'bar' } };
    rerender(props);
    await act(async () => result.current.fetch());
    expect(resourceNames(result)).toEqual(['bar0', 'bar1']);

    // Make sure we reached the end of the data set.
    await act(async () => result.current.fetch());
    props = { ...props, clusterId: 'cluster3' };
    rerender(props);
    await act(async () => result.current.fetch());
    expect(resourceClusterIds(result)).toEqual(['cluster3', 'cluster3']);
  });

  it("doesn't fetch if params didn't change on rerender", async () => {
    const { result, rerender } = renderHook(useKeyBasedPagination, {
      initialProps: hookProps(),
    });
    await act(async () => result.current.fetch());
    expect(resourceNames(result)).toEqual(['r0', 'r1']);
    await act(async () => rerender(hookProps()));
    expect(resourceNames(result)).toEqual(['r0', 'r1']);
  });

  describe("doesn't react to fetch() calls before the previous one finishes", () => {
    let props, fetchSpy, result;

    beforeEach(() => {
      props = hookProps();
      fetchSpy = jest.spyOn(props, 'fetchFunc');
      ({ result } = renderHook(() => useKeyBasedPagination(props)));
    });

    test('when called once per state reconciliation cycle', async () => {
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

  it('cancels pending request if params change', async () => {
    let props = hookProps({ clusterId: 'cluster1' });
    const { result, rerender } = renderHook(useKeyBasedPagination, {
      initialProps: props,
    });
    let fetchPromise;
    act(() => {
      fetchPromise = result.current.fetch();
    });
    rerender({ ...props, clusterId: 'cluster2' });
    await act(async () => fetchPromise);
    expect(resourceClusterIds(result)).toEqual([]);
    await act(async () => result.current.fetch());
    expect(resourceClusterIds(result)).toEqual(['cluster2', 'cluster2']);
  });
});
