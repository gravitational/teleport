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

import { act, renderHook, waitFor } from '@testing-library/react';

import { wait } from 'shared/utils/wait';

import {
  Attempt,
  CanceledError,
  useAsync,
  useDelayedRepeatedAttempt,
} from './useAsync';

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

  const [, error] = await promise;
  await expect(
    error instanceof CanceledError && error.stalePromise
  ).resolves.toEqual(expect.any(Symbol));
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

  const [, error] = await promise;
  await expect(
    error instanceof CanceledError && error.stalePromise
  ).rejects.toThrow(expect.objectContaining({ message: 'oops' }));
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

  const [, error] = await firstRunPromise;
  await expect(
    error instanceof CanceledError && error.stalePromise
  ).resolves.toEqual(1);
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

  const [, error] = await firstRunPromise;
  await expect(
    error instanceof CanceledError && error.stalePromise
  ).rejects.toThrow(expect.objectContaining({ message: 'oops 1' }));
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

describe('useDelayedRepeatedAttempt', () => {
  it('does not update attempt status if it resolves before delay', async () => {
    let resolve: (symbol: symbol) => void;
    const { result } = renderHook(() => {
      const [eagerAttempt, run] = useAsync(() => {
        // Resolve to a symbol so that successful attempts are not equal to each other.
        return new Promise<symbol>(res => {
          resolve = res;
        });
      });
      const delayedAttempt = useDelayedRepeatedAttempt(eagerAttempt, 500);

      return { run, delayedAttempt };
    });

    await act(async () => {
      const promise = result.current.run();
      resolve(Symbol());
      await promise;
    });

    expect(result.current.delayedAttempt.status).toEqual('success');
    const oldDelayedAttempt = result.current.delayedAttempt;

    act(() => {
      // Start promise but do not await it, instead wait for attempt updates. This ensures that we
      // catch any state updates caused by the promise being resolved.
      result.current.run();
      resolve(Symbol());
    });

    // Wait for delayedAttempt to get updated.
    let nextDelayedAttempt: Attempt<symbol>;
    await waitFor(
      () => {
        // result.current always returns the current result. Capture the attempt that's being
        // compared within waitFor so that we can check its status outside of the block and be sure
        // that it's the same attempt.
        nextDelayedAttempt = result.current.delayedAttempt;
        expect(nextDelayedAttempt).not.toBe(oldDelayedAttempt);
      },
      {
        onTimeout: error => {
          error.message =
            'delayedAttempt did not get updated within timeout. ' +
            `This might mean that the logic for detecting attempt updates is incorrect.\n${error.message}`;
          return error;
        },
      }
    );

    // As the promise was resolved before the delay, the attempt status should still be success.
    expect(nextDelayedAttempt.status).toEqual('success');
  });

  it('updates attempt status to processing if it does not resolve before delay', async () => {
    let resolve: (symbol: symbol) => void;
    const { result } = renderHook(() => {
      const [eagerAttempt, run] = useAsync(() => {
        // Resolve to a symbol so that successful attempts are not equal to each other.
        return new Promise<symbol>(res => {
          resolve = res;
        });
      });
      const delayedAttempt = useDelayedRepeatedAttempt(eagerAttempt, 100);

      return { run, delayedAttempt };
    });

    await act(async () => {
      const promise = result.current.run();
      resolve(Symbol());
      await promise;
    });

    expect(result.current.delayedAttempt.status).toEqual('success');
    const oldDelayedAttempt = result.current.delayedAttempt;

    let promise: Promise<[symbol, Error]>;
    act(() => {
      // Start promise but do not resolve it.
      promise = result.current.run();
    });

    // Wait for delayedAttempt to get updated.
    let nextDelayedAttempt: Attempt<symbol>;
    await waitFor(() => {
      // result.current always returns the current result. Capture the attempt that's being compared
      // within waitFor so that we can check its status outside of the block and be sure that it's
      // the same attempt.
      nextDelayedAttempt = result.current.delayedAttempt;
      expect(nextDelayedAttempt).not.toBe(oldDelayedAttempt);
    });
    expect(nextDelayedAttempt.status).toEqual('processing');

    await act(async () => {
      // Resolve the promise after the status was updated to processing.
      resolve(Symbol());
      await promise;
    });
    expect(result.current.delayedAttempt.status).toEqual('success');
  });

  it('cancels pending update', async () => {
    const delayMs = 100;
    let resolve: (symbol: symbol) => void;
    const { result } = renderHook(() => {
      const [eagerAttempt, run] = useAsync(() => {
        // Resolve to a symbol so that successful attempts are not equal to each other.
        return new Promise<symbol>(res => {
          resolve = res;
        });
      });
      const delayedAttempt = useDelayedRepeatedAttempt(eagerAttempt, delayMs);

      return { run, delayedAttempt, eagerAttempt };
    });

    await act(async () => {
      const promise = result.current.run();
      resolve(Symbol());
      await promise;
    });

    expect(result.current.delayedAttempt.status).toEqual('success');

    let promise: Promise<[symbol, Error]>;
    act(() => {
      // Start promise but do not resolve it.
      promise = result.current.run();
    });

    // The _eager_ attempt gets updated to a processing state.
    // This means that the hook now enqueued a delayed update of delayedAttempt.
    expect(result.current.eagerAttempt.status).toEqual('processing');
    expect(result.current.delayedAttempt.status).toEqual('success');

    await act(async () => {
      // Resolve the promise. This transitions eagerAttempt and delayedAttempt to a success state.
      resolve(Symbol());
      await promise;
    });

    expect(result.current.eagerAttempt.status).toEqual('success');
    expect(result.current.delayedAttempt.status).toEqual('success');

    // Wait until the delay. If the pending update was not properly canceled, this should execute
    // it. As such, this will count as a state update outside of `act`, which will surface an error.
    //
    // In case the update does not get canceled, delayedAttempt will not get updated to pending.
    // The pending update will call setCurrentAttempt, which will set currentAttempt to processing.
    // The hook will reexecute and the effect will set currentAttempt back to success since
    // currentAttempt != attempt. This is all because at the time when the pending update gets
    // erroneously executed, eagerAttempt is already successful.
    await wait(delayMs);
  });
});
