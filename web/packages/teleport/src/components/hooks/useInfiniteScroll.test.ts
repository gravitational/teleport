import React from 'react';
import { renderHook, act, RenderResult } from '@testing-library/react-hooks';
import { useKeyBasedPagination, State } from './useInfiniteScroll';
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
  return async (clusterId: string, params: UrlResourcesParams) => {
    const { startKey, limit } = params;
    const startIndex = parseInt(startKey || '0');
    const namePrefix = params.search ?? 'r';
    const endIndex = startIndex + limit;
    const nextStartKey =
      endIndex < numResources ? endIndex.toString() : undefined;
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

describe('useKeyBasedPagination', () => {
  it.each`
    n     | names
    ${3}  | ${['r0', 'r1', 'r2']}
    ${4}  | ${['r0', 'r1', 'r2', 'r3']}
    ${10} | ${['r0', 'r1', 'r2', 'r3']}
  `('Fetches one data batch, n=$n', async ({ n, names }) => {
    const { result } = renderHook(() =>
      useKeyBasedPagination({
        fetchFunc: newFetchFunc(4),
        clusterId: 'test-cluster',
        filter: {},
        initialFetchSize: n,
      })
    );

    expect(result.current.fetchedData.agents).toEqual([]);
    await act(async () => result.current.fetch());
    expect(resourceNames(result)).toEqual(names);
  });

  it('fetches multiple data batches', async () => {
    const { result } = renderHook(() =>
      useKeyBasedPagination({
        fetchFunc: newFetchFunc(7),
        clusterId: 'test-cluster',
        filter: {},
        initialFetchSize: 2,
        fetchMoreSize: 3,
      })
    );

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
    const { result } = renderHook(() =>
      useKeyBasedPagination({
        fetchFunc: newFetchFunc(7),
        clusterId: 'test-cluster',
        filter: {},
        initialFetchSize: 2,
        fetchMoreSize: 3,
      })
    );

    expect(result.current.attempt.status).toBe('');
    let fetchResult;
    act(() => {
      fetchResult = result.current.fetch();
    });
    expect(result.current.attempt.status).toBe('processing');
    await act(async () => fetchResult);
    expect(result.current.attempt.status).toBe('success');

    act(() => {
      fetchResult = result.current.fetch();
    });
    expect(result.current.attempt.status).toBe('processing');
    await act(async () => fetchResult);
    expect(result.current.attempt.status).toBe('success');
  });

  it('restarts after query params change', async () => {
    let props = {
      fetchFunc: newFetchFunc(4),
      clusterId: 'cluster1',
      filter: { search: 'foo' },
      initialFetchSize: 2,
      fetchMoreSize: 3,
    };
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
});
