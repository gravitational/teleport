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

import { useCallback, useEffect, useRef, useState } from 'react';

/**
 * `useAsync` lets you represent the state of an async operation as data. It accepts an async function
 * that you want to execute. Calling the hook returns an array of three elements:
 *
 * * The first element is the representation of the attempt to run that async function as data, the
 *   so called attempt object.
 * * The second element is a function which when called starts to execute the async function (the
 * `run` function).
 * * The third element is a function that lets you directly update the attempt object if needed.
 *
 * `useAsync` automatically ignores stale promises either when the underlying component gets
 * unmounted. If the `run` function gets executed again while the promise from the previous
 * execution still hasn't finished, the return value of the previous promise will be ignored.
 *
 * The primary interface through which you should interact with the result of the callback is the
 * attempt object. The `run` function has a return value as well which is useful if you need to
 * chain its result with some other action inside an event handler or in `useEffect`. The return
 * value of the `run` function corresponds to the return value of that specific invocation of the
 * callback passed to `useAsync`. This means you need to manually handle the `CanceledError` case.
 *
 * @example
 * export function useUserProfile(userId) {
 *   const [fetchUserProfileAttempt, fetchUserProfile] = useAsync(async () => {
 *     return await fetch(`/users/${userId}`);
 *   })
 *
 *   return { fetchUserProfileAttempt, fetchUserProfile };
 * }
 *
 * @example In the view layer you can use it like this:
 * function UserProfile(props) {
 *   const { fetchUserProfileAttempt, fetchUserProfile } = useUserProfile(props.id);
 *
 *   useEffect(() => {
 *     if (!fetchUserProfileAttempt.status) {
 *       fetchUserProfile()
 *     }
 *   }, [fetchUserProfileAttempt, fetchUserProfile])
 *
 *    switch (fetchUserProfileAttempt.status) {
 *      case '':
 *      case 'processing':
 *        return <Spinner />;
 *      case 'error':
 *        return <ErrorMessage text={fetchUserProfileAttempt.statusText} />;
 *      case 'success':
 *       return <UserAvatar url={fetchUserProfileAttempt.data.avatarUrl} />;
 *    }
 * }
 *
 * @example Consuming the `run` return value
 * function UserProfile(props) {
 *   const { fetchUserProfileAttempt, fetchUserProfile } = useUserProfile(props.id);
 *
 *   useEffect(async () => {
 *     if (!fetchUserProfileAttempt.status) {
 *       const [profile, err] = fetchUserProfile()
 *
 *       if (err && !(err instanceof CanceledError)) {
 *         // Handle the error.
 *       }
 *     }
 *   }, [fetchUserProfileAttempt, fetchUserProfile])
 * }
 *
 */
export function useAsync<Args extends unknown[], AttemptData>(
  cb: (...args: Args) => Promise<AttemptData>
) {
  const [state, setState] = useState<Attempt<AttemptData>>(makeEmptyAttempt);
  const isMounted = useIsMounted();
  const asyncTask = useRef<Promise<AttemptData>>();

  const run: (...args: Args) => RunFuncReturnValue<AttemptData> = useCallback(
    (...args: Args) => {
      setState(prevState => ({
        status: 'processing',
        data: prevState['data'],
        statusText: prevState['statusText'],
      }));

      const promise = cb(...args);
      asyncTask.current = promise;

      return promise.then(
        data => {
          if (!isMounted()) {
            return [null, new CanceledError(promise)] as [AttemptData, Error];
          }
          if (asyncTask.current !== promise) {
            return [null, new CanceledError(promise)] as [AttemptData, Error];
          }

          setState(prevState => ({
            ...prevState,
            status: 'success',
            data,
          }));

          return [data, null] as [AttemptData, Error];
        },
        err => {
          if (!isMounted()) {
            return [null, new CanceledError(promise)] as [AttemptData, Error];
          }
          if (asyncTask.current !== promise) {
            return [null, new CanceledError(promise)] as [AttemptData, Error];
          }

          setState(() => ({
            status: 'error',
            error: err,
            statusText: err?.message,
            data: null,
          }));

          return [null, err] as [AttemptData, Error];
        }
      );
    },
    [setState, cb, isMounted]
  );

  return [state, run, setState] as const;
}

function useIsMounted() {
  const isMounted = useRef(false);

  useEffect(() => {
    isMounted.current = true;

    return () => {
      isMounted.current = false;
    };
  }, []);

  return useCallback(() => isMounted.current, []);
}

export class CanceledError<AttemptData> extends Error {
  constructor(
    /**
     * stalePromise is the promise which result was ignored because another useAsync run was
     * started. This gives the callsite a chance to use a result from this stale run, even after
     * another run was started.
     */
    public stalePromise?: Promise<AttemptData>
  ) {
    super('Ignored response from stale useAsync request');
    this.name = 'CanceledError';
  }
}

export type AttemptStatus = Attempt<any>['status'];

export type Attempt<T> =
  | {
      status: '';
      data: null;
      /**
       * @deprecated statusText is present for compatibility purposes only. To use statusText, check
       * if status equals 'error' first.
       */
      statusText: string;
    }
  | {
      status: 'processing';
      /** data is either null or contains data from the previous success attempt if the attempt was retried. */
      data: null | T;
      /**
       * @deprecated statusText is present for compatibility purposes only. To use statusText, check
       * if status equals 'error' first.
       */
      statusText: string;
    }
  | {
      status: 'success';
      data: T;
      /**
       * @deprecated statusText is present for compatibility purposes only. To use statusText, check
       * if status equals 'error' first.
       */
      statusText: string;
    }
  | {
      status: 'error';
      data: null;
      statusText: string;
      error: any;
    };

export function hasFinished<T>(attempt: Attempt<T>): boolean {
  return attempt.status === 'success' || attempt.status === 'error';
}

export function makeEmptyAttempt<T>(): Attempt<T> {
  return {
    data: null,
    status: '',
    statusText: '',
  };
}

export function makeSuccessAttempt<T>(data: T): Attempt<T> {
  return {
    data,
    status: 'success',
    statusText: '',
  };
}

export function makeProcessingAttempt<T>(): Attempt<T> {
  return {
    data: null,
    status: 'processing',
    statusText: '',
  };
}

export function makeErrorAttempt<T>(error: Error): Attempt<T> {
  return {
    data: null,
    status: 'error',
    error: error,
    statusText: error.message,
  };
}

/**
 * @deprecated Use makeErrorAttempt instead.
 */
export function makeErrorAttemptWithStatusText<T>(
  statusText: string
): Attempt<T> {
  return {
    data: null,
    status: 'error',
    statusText,
    error: new Error(statusText),
  };
}

/**
 * mapAttempt maps attempt data if the attempt is successful or in progress and contains data.
 */
export function mapAttempt<A, B>(
  attempt: Attempt<A>,
  mapFunction: (attemptData: A) => B
): Attempt<B> {
  if (
    attempt.status === 'success' ||
    (attempt.status === 'processing' && attempt.data)
  ) {
    return {
      ...attempt,
      data: mapFunction(attempt.data),
    };
  }

  return {
    ...attempt,
    data: null,
  };
}

/**
 * useDelayedRepeatedAttempt makes it so that on repeated calls to `run`, the attempt changes its
 * state to 'processing' only after a delay. This can be used to mask repeated calls and
 * optimistically show stale results.
 *
 * @example
 * const [eagerFetchUserProfileAttempt, fetchUserProfile] = useAsync(async () => {
 *   return await fetch(`/users/${userId}`);
 * })
 * const fetchUserProfileAttempt = useDelayedRepeatedAttempt(eagerFetchUserProfileAttempt, 600)
 */
export function useDelayedRepeatedAttempt<Data>(
  attempt: Attempt<Data>,
  delayMs = 400
): Attempt<Data> {
  const [currentAttempt, setCurrentAttempt] = useState(attempt);

  useEffect(() => {
    if (
      currentAttempt.status === 'success' &&
      attempt.status === 'processing'
    ) {
      const timeout = setTimeout(() => {
        setCurrentAttempt(attempt);
      }, delayMs);
      return () => {
        clearTimeout(timeout);
      };
    }

    if (currentAttempt !== attempt) {
      setCurrentAttempt(attempt);
    }
  }, [attempt, currentAttempt, delayMs]);

  return currentAttempt;
}

export type RunFuncReturnValue<AttemptData> = Promise<[AttemptData, Error]>;
