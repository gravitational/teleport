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

import { renderHook, act, waitFor } from '@testing-library/react';

import { useAsync, CanceledError } from './useAsync';

test('run returns a promise which resolves with the attempt data', async () => {
  const returnValue = Symbol();
  const { result } = renderHook(() =>
    useAsync(() => Promise.resolve(returnValue))
  );

  let [, run] = result.current;
  let promise: Promise<[symbol, Error]>;
  act(() => {
    promise = run();
  });

  await waitFor(() => expect(promise).resolves.toEqual([returnValue, null]));
});

test('run resolves the promise to an error and does not update the state on unmount when the callback returns a resolved promise', async () => {
  const { result, unmount } = renderHook(() =>
    useAsync(() => Promise.resolve(Symbol()))
  );

  let [, run] = result.current;
  let promise: Promise<[symbol, Error]>;
  act(() => {
    promise = run();
  });
  unmount();

  await expect(promise).resolves.toEqual([null, new CanceledError()]);
  const [attempt] = result.current;
  expect(attempt.status).toBe('processing');
});

test('run resolves the promise to an error and does not update the state on unmount when the callback returns a rejected promise', async () => {
  const { result, unmount } = renderHook(() =>
    useAsync(() => Promise.reject(new Error('oops')))
  );

  let [, run] = result.current;
  let promise: Promise<[symbol, Error]>;
  act(() => {
    promise = run();
  });
  unmount();

  await expect(promise).resolves.toEqual([null, new CanceledError()]);
  const [attempt] = result.current;
  expect(attempt.status).toBe('processing');
});

test('run resolves the promise to an error after being re-run when the callback returns a resolved promise', async () => {
  const { result } = renderHook(() =>
    useAsync((count: number) => Promise.resolve(count))
  );

  let [, run] = result.current;
  let firstRunPromise: Promise<[number, Error]>;
  act(() => {
    firstRunPromise = run(1);
  });

  act(() => {
    run(2);
  });

  await waitFor(() =>
    expect(firstRunPromise).resolves.toEqual([null, new CanceledError()])
  );
});

test('run does not update state after being re-run when the callback returns a resolved promise', async () => {
  let resolveFirstPromise, resolveSecondPromise;
  const firstPromise = new Promise(resolve => {
    resolveFirstPromise = resolve;
  });
  const secondPromise = new Promise(resolve => {
    resolveSecondPromise = resolve;
  });

  const { result } = renderHook(() =>
    // Passing a promise lets us control when the callback resolves.
    useAsync((promise: Promise<unknown>) => promise)
  );

  let firstRunPromise: Promise<[unknown, Error]>;
  let secondRunPromise: Promise<[unknown, Error]>;

  let [, run] = result.current;
  await act(async () => {
    // Start two runs, one after the other.
    firstRunPromise = run(firstPromise);
    secondRunPromise = run(secondPromise);

    // Once the first promise resolves, it should see that another one was started. The first
    // promise should return early with an error.
    resolveFirstPromise();
    await firstRunPromise;
  });

  const attemptAfterFirstPromise = result.current[0];

  await waitFor(() =>
    expect(attemptAfterFirstPromise.status).toBe('processing')
  );

  await act(async () => {
    resolveSecondPromise();
    await secondRunPromise;
  });
});

test('run resolves the promise to an error after being re-run when the callback returns a rejected promise', async () => {
  const { result } = renderHook(() =>
    useAsync((count: number) => Promise.reject(new Error(`oops ${count}`)))
  );

  let [, run] = result.current;
  let firstRunPromise: Promise<[number, Error]>;
  act(() => {
    firstRunPromise = run(1);
  });

  act(() => {
    run(2);
  });

  await waitFor(() =>
    expect(firstRunPromise).resolves.toEqual([null, new CanceledError()])
  );
});

test('run does not update state after being re-run when the callback returns a rejected promise', async () => {
  let rejectFirstPromise, rejectSecondPromise;
  const firstPromise = new Promise((resolve, reject) => {
    rejectFirstPromise = reject;
  });
  const secondPromise = new Promise((resolve, reject) => {
    rejectSecondPromise = reject;
  });

  const { result } = renderHook(() =>
    // Passing a promise lets us control when the callback resolves.
    useAsync((promise: Promise<unknown>) => promise)
  );

  let firstRunPromise: Promise<[unknown, Error]>;
  let secondRunPromise: Promise<[unknown, Error]>;

  let [, run] = result.current;
  await act(async () => {
    // Start two runs, one after the other.
    firstRunPromise = run(firstPromise);
    secondRunPromise = run(secondPromise);

    // Once the first promise resolves, it should see that another one was started. The first
    // promise should return early with an error.
    rejectFirstPromise();
    await firstRunPromise;
  });

  const attemptAfterFirstPromise = result.current[0];
  await waitFor(() =>
    expect(attemptAfterFirstPromise.status).toBe('processing')
  );

  await act(async () => {
    rejectSecondPromise();
    await secondRunPromise;
  });
});

test('error and statusText are set when the callback returns a rejected promise', async () => {
  const expectedError = new Error('whoops');

  const { result } = renderHook(() =>
    useAsync(() => Promise.reject(expectedError))
  );

  let runPromise: Promise<any>;
  let [, run] = result.current;

  act(() => {
    runPromise = run();
  });
  await waitFor(() => expect(result.current[0].status).toBe('error'));

  const attempt = result.current[0];
  expect(attempt['error']).toBe(expectedError);
  expect(attempt['statusText']).toEqual(expectedError.message);

  // The promise returned from run always succeeds, but any errors are captured as the second arg.
  await expect(runPromise).resolves.toEqual([null, expectedError]);
});
