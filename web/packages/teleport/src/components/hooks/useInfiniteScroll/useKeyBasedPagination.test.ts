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

import { renderHook, act } from '@testing-library/react-hooks';

import { ApiError } from 'teleport/services/api/parseError';

import { Node } from 'teleport/services/nodes';

import { useKeyBasedPagination, Props } from './useKeyBasedPagination';
import {
  newApiAbortError,
  newDOMAbortError,
  newFetchFunc,
  resourceClusterIds,
  resourceNames,
} from './testUtils';

function hookProps(overrides: Partial<Props<Node>> = {}) {
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
  const { result } = renderHook(useKeyBasedPagination, {
    initialProps: hookProps({
      fetchFunc: newFetchFunc(4),
      initialFetchSize: n,
    }),
  });

  expect(result.current.resources).toEqual([]);
  await act(result.current.fetch);
  expect(resourceNames(result)).toEqual(names);
});

test('fetches multiple data batches', async () => {
  const { result } = renderHook(useKeyBasedPagination, {
    initialProps: hookProps(),
  });
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
  const { result } = renderHook(useKeyBasedPagination, {
    initialProps: hookProps(),
  });

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
  const props = hookProps();
  const { result, rerender } = renderHook(useKeyBasedPagination, {
    initialProps: props,
  });
  await act(result.current.fetch);
  expect(resourceNames(result)).toEqual(['r0', 'r1']);
  rerender(props);
  await act(result.current.fetch);
  expect(resourceNames(result)).toEqual(['r0', 'r1', 'r2', 'r3', 'r4']);
});

describe("doesn't react to fetch() calls before the previous one finishes", () => {
  let props: Props<Node>, fetchSpy;

  beforeEach(() => {
    props = hookProps();
    fetchSpy = jest.spyOn(props, 'fetchFunc');
  });

  test('when called once per state reconciliation cycle', async () => {
    const { result } = renderHook(useKeyBasedPagination, {
      initialProps: props,
    });
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
    const { result } = renderHook(useKeyBasedPagination, {
      initialProps: props,
    });
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
    const { result } = renderHook(useKeyBasedPagination, {
      initialProps: props,
    });
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
    const { result } = renderHook(useKeyBasedPagination, {
      initialProps: props,
    });
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

test('forceFetch spawns another request, even if there is one pending', async () => {
  const props = hookProps();
  const fetchSpy = jest.spyOn(props, 'fetchFunc');
  const { result } = renderHook(useKeyBasedPagination, {
    initialProps: props,
  });
  fetchSpy.mockImplementationOnce(async () => {
    return {
      agents: [
        {
          kind: 'node',
          id: 'sus',
          clusterId: 'test-cluster',
          hostname: `impostor`,
          labels: [],
          addr: '',
          tunnel: false,
          sshLogins: [],
        },
      ],
    };
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

test("doesn't get confused if aborting a request still results in a successful promise", async () => {
  // This one is tricky. It turns out that somewhere in our API layer, we
  // perform some asynchronous operation that disregards the abort signal.
  // Whether it's because some platform implementation doesn't adhere to the
  // spec, or whether we miss some detail - all in all, in the principle, looks
  // like this hook can't really trust the abort signal to be 100% effective.
  let props = hookProps({
    // Create a function that will never throw an abort error.
    fetchFunc: newFetchFunc(1, () => null),
    filter: { search: 'rabbit' },
  });
  const { result, rerender } = renderHook(useKeyBasedPagination, {
    initialProps: props,
  });
  await act(result.current.fetch);
  expect(resourceNames(result)).toEqual(['rabbit0']);

  let f1, f2;
  props = { ...props, filter: { search: 'duck' } };
  rerender(props);
  act(() => {
    f1 = result.current.fetch();
  });

  props = { ...props, filter: { search: 'rabbit' } };
  rerender(props);
  act(() => {
    f2 = result.current.fetch();
  });

  await act(async () => Promise.all([f1, f2]));
  expect(resourceNames(result)).toEqual(['rabbit0']);
});
