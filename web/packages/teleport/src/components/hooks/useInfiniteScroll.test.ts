import React from 'react';
import { renderHook, act, RenderResult } from '@testing-library/react-hooks';
import { useInfiniteScroll, State } from './useInfiniteScroll';
import { UrlResourcesParams } from 'teleport/config';

interface TestResource {
  name: string;
}

function getTestResources(n: number): TestResource[] {
  return Array(n)
    .fill(0)
    .map((_, i) => ({ name: `r${i}` }));
}

function newFetchFunc(numResources: number) {
  return async (clusterId: string, params: UrlResourcesParams) => {
    const { startKey, limit } = params;
    const startIndex = parseInt(startKey || '0');
    const endIndex = startIndex + limit;
    const nextStartKey =
      endIndex < numResources ? endIndex.toString() : undefined;
    return {
      agents: getTestResources(numResources).slice(
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

test.each`
  n     | names
  ${3}  | ${['r0', 'r1', 'r2']}
  ${4}  | ${['r0', 'r1', 'r2', 'r3']}
  ${10} | ${['r0', 'r1', 'r2', 'r3']}
`('Fetches one data batch, n=$n', async ({ n, names }) => {
  const { result } = renderHook(() =>
    useInfiniteScroll({
      fetchFunc: newFetchFunc(4),
      clusterId: 'test-cluster',
      params: {},
      initialFetchSize: n,
    })
  );

  expect(result.current.fetchedData.agents).toEqual([]);
  await act(async () => result.current.fetchInitial());
  expect(resourceNames(result)).toEqual(names);
});

test('Fetches multiple data batches', async () => {
  const { result } = renderHook(() =>
    useInfiniteScroll({
      fetchFunc: newFetchFunc(7),
      clusterId: 'test-cluster',
      params: {},
      initialFetchSize: 2,
      fetchMoreSize: 3,
    })
  );

  await act(async () => result.current.fetchInitial());
  await act(async () => result.current.fetchMore());
  expect(resourceNames(result)).toEqual(['r0', 'r1', 'r2', 'r3', 'r4']);
  await act(async () => result.current.fetchMore());

  const allResources = ['r0', 'r1', 'r2', 'r3', 'r4', 'r5', 'r6'];
  expect(resourceNames(result)).toEqual(allResources);
  await act(async () => result.current.fetchMore());
  expect(resourceNames(result)).toEqual(allResources);
});

test('Maintains attempt state', async () => {
  const { result } = renderHook(() =>
    useInfiniteScroll({
      fetchFunc: newFetchFunc(7),
      clusterId: 'test-cluster',
      params: {},
      initialFetchSize: 2,
      fetchMoreSize: 3,
    })
  );

  expect(result.current.attempt.status).toBe('processing');
  let fetchResult;
  act(() => {
    fetchResult = result.current.fetchInitial();
  });
  expect(result.current.attempt.status).toBe('processing');
  await act(async () => fetchResult);
  expect(result.current.attempt.status).toBe('success');

  act(() => {
    fetchResult = result.current.fetchMore();
  });
  expect(result.current.attempt.status).toBe('processing');
  await act(async () => fetchResult);
  expect(result.current.attempt.status).toBe('success');
});
