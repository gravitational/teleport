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

import { useCallback, useState, useRef, useEffect } from 'react';

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
 *
 */
export function useAsync<Args extends unknown[], AttemptData>(
  cb: (...args: Args) => Promise<AttemptData>
) {
  const [state, setState] = useState<Attempt<AttemptData>>(makeEmptyAttempt);
  const isMounted = useIsMounted();
  const asyncTask = useRef<Promise<AttemptData>>();

  const run = useCallback(
    (...args: Args) => {
      setState(prevState => ({
        ...prevState,
        status: 'processing',
      }));

      const promise = cb(...args);
      asyncTask.current = promise;

      return promise.then(
        data => {
          if (!isMounted()) {
            return [null, new CanceledError()] as [AttemptData, Error];
          }
          if (asyncTask.current !== promise) {
            return [null, new CanceledError()] as [AttemptData, Error];
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
            return [null, new CanceledError()] as [AttemptData, Error];
          }
          if (asyncTask.current !== promise) {
            return [null, new CanceledError()] as [AttemptData, Error];
          }

          setState(prevState => ({
            ...prevState,
            status: 'error',
            statusText: err?.message,
            data: null,
          }));

          return [null, err] as [AttemptData, Error];
        }
      );
    },
    [setState, cb, isMounted]
  );

  const setAttempt = useCallback(
    (attempt: Attempt<AttemptData>) => {
      setState(attempt);
    },
    [setState]
  );

  return [state, run, setAttempt] as const;
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

export class CanceledError extends Error {
  constructor() {
    super('Ignored response from stale useAsync request');
    this.name = 'CanceledError';
  }
}

export type AttemptStatus = 'processing' | 'success' | 'error' | '';

export type Attempt<T> = {
  data?: T;
  status: AttemptStatus;
  statusText: string;
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

export function makeErrorAttempt<T>(statusText: string): Attempt<T> {
  return {
    data: null,
    status: 'error',
    statusText,
  };
}
