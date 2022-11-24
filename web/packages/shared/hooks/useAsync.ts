/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/* eslint-disable @typescript-eslint/ban-types */

import { useCallback, useState } from 'react';

/**
 * `useAsync` lets you represent the state of an async operation as data. It accepts an async function
 * that you want to execute. Calling the hook returns an array of three elements:
 *
 * * The first element is the representation of the attempt to run that async function as data, the
 *   so called attempt object.
 * * The second element is a function which when called starts to execute the async function.
 * * The third element is a function that lets you directly update the attempt object if needed.
 *
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
 *
 * @example In the view layer you can use it like this:
 * function UserProfile(props) {
 *   const { fetchUserProfileAttempt, fetchUserProfile } = useUserProfile(props.id);
 *
 *   useEffect(() => {
 *     if (!fetchUserProfileAttempt.status) {
 *       fetchUserProfile()
 *     }
 *   }, [fetchUserProfileAttempt])
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
 */
export function useAsync<R, T extends Function>(cb?: AsyncCb<R, T>) {
  const [state, setState] = useState<Attempt<R>>(makeEmptyAttempt);

  const run = useCallback(
    (...p: Parameters<AsyncCb<R, T>>) =>
      Promise.resolve()
        .then(() => {
          setState(prevState => ({
            ...prevState,
            status: 'processing',
          }));

          return cb.call(null, ...p) as R;
        })
        .then(
          data => {
            setState(prevState => ({
              ...prevState,
              status: 'success',
              data,
            }));

            return [data, null] as [R, Error];
          },
          err => {
            setState(prevState => ({
              ...prevState,
              status: 'error',
              statusText: err?.message,
              data: null,
            }));

            return [null, err] as [R, Error];
          }
        ),
    [setState, cb]
  );

  const setAttempt = useCallback(
    (attempt: Attempt<R>) => {
      setState(attempt);
    },
    [setState]
  );

  return [state, run, setAttempt] as const;
}

export type AttemptStatus = 'processing' | 'success' | 'error' | '';

export type Attempt<T> = {
  data?: T;
  status: AttemptStatus;
  statusText: string;
};

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

export function makeErrorAttempt<T>(statusText: string): Attempt<T> {
  return {
    data: null,
    status: 'error',
    statusText,
  };
}

type IsValidArg<T> = T extends object
  ? keyof T extends never
    ? false
    : true
  : true;

type AsyncCb<R, T extends Function> = T extends (...args: any[]) => Promise<any>
  ? T
  : T extends (
      a: infer A,
      b: infer B,
      c: infer C,
      d: infer D,
      e: infer E,
      f: infer F,
      g: infer G,
      h: infer H,
      i: infer I,
      j: infer J
    ) => Promise<R>
  ? IsValidArg<J> extends true
    ? (a: A, b: B, c: C, d: D, e: E, f: F, g: G, h: H, i: I, j: J) => Promise<R>
    : IsValidArg<I> extends true
    ? (a: A, b: B, c: C, d: D, e: E, f: F, g: G, h: H, i: I) => Promise<R>
    : IsValidArg<H> extends true
    ? (a: A, b: B, c: C, d: D, e: E, f: F, g: G, h: H) => Promise<R>
    : IsValidArg<G> extends true
    ? (a: A, b: B, c: C, d: D, e: E, f: F, g: G) => Promise<R>
    : IsValidArg<F> extends true
    ? (a: A, b: B, c: C, d: D, e: E, f: F) => Promise<R>
    : IsValidArg<E> extends true
    ? (a: A, b: B, c: C, d: D, e: E) => Promise<R>
    : IsValidArg<D> extends true
    ? (a: A, b: B, c: C, d: D) => Promise<R>
    : IsValidArg<C> extends true
    ? (a: A, b: B, c: C) => Promise<R>
    : IsValidArg<B> extends true
    ? (a: A, b: B) => Promise<R>
    : IsValidArg<A> extends true
    ? (a: A) => Promise<R>
    : () => Promise<R>
  : never;
