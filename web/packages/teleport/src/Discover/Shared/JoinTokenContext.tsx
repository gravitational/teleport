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

import React, { useCallback, useContext, useEffect, useState } from 'react';

import { useTeleport } from 'teleport';

import {
  ResourceKind,
  resourceKindToJoinRole,
} from 'teleport/Discover/Shared/ResourceKind';

import type { JoinToken, JoinMethod } from 'teleport/services/joinToken';

interface JoinTokenContextState {
  joinToken: JoinToken;
  setJoinToken: (joinToken: JoinToken) => void;
  timeout: number;
  timedOut: boolean;
  startTimer: () => void;
  id?: number;
}

const joinTokenContext = React.createContext<JoinTokenContextState>(null);

export function JoinTokenProvider(props: {
  timeout: number;
  children?: React.ReactNode;
}) {
  const [joinToken, setJoinToken] = useState<JoinToken>(null);
  const [timedOut, setTimedOut] = useState(false);
  const [timeout, setTokenTimeout] = useState<number>(null);

  useEffect(() => {
    if (!timeout) {
      return;
    }

    if (timeout > Date.now()) {
      setTimedOut(false);

      const id = window.setTimeout(
        () => setTimedOut(true),
        timeout - Date.now()
      );

      return () => clearTimeout(id);
    }
  }, [timeout]);

  const startTimer = useCallback(() => {
    setTokenTimeout(Date.now() + props.timeout);
  }, [props.timeout]);

  return (
    <joinTokenContext.Provider
      value={{ joinToken, setJoinToken, timeout, startTimer, timedOut }}
    >
      {props.children}
    </joinTokenContext.Provider>
  );
}

interface SuspendResult {
  promise?: Promise<any>;
  response?: JoinToken;
  error?: Error;
}

let abortController: AbortController;
let joinTokenCache = new Map<ResourceKind, SuspendResult>();

export function clearCachedJoinTokenResult(resourceKind: ResourceKind) {
  joinTokenCache.delete(resourceKind);
}

export function useJoinTokenValue() {
  const tokenContext = useContext(joinTokenContext);

  return tokenContext.joinToken;
}

export function useJoinToken(
  resourceKind: ResourceKind,
  runNow = true,
  joinMethod: JoinMethod = 'token'
): {
  joinToken: JoinToken;
  reloadJoinToken: () => void;
  timedOut: boolean;
  timeout: number;
} {
  const ctx = useTeleport();
  const tokenContext = useContext(joinTokenContext);

  const [, rerender] = useState(0);

  function run() {
    abortController = new AbortController();

    const result: SuspendResult = {
      response: null,
      error: null,
      promise: ctx.joinTokenService
        .fetchJoinToken(
          [resourceKindToJoinRole(resourceKind)],
          joinMethod,
          [],
          abortController.signal
        )
        .then(token => {
          // Probably will never happen, but just in case, otherwise
          // querying for the resource can return a false positive.
          if (!token.internalResourceId) {
            throw new Error(
              'internal resource ID is required to discover the newly added resource, but none was provided'
            );
          }
          result.response = token;
          tokenContext.setJoinToken(token);
          tokenContext.startTimer();
        })
        .catch(error => {
          result.error = error;
        }),
    };

    joinTokenCache.set(resourceKind, result);

    return result;
  }

  useEffect(() => {
    return () => {
      abortController?.abort();
    };
  }, []);

  const existing = joinTokenCache.get(resourceKind);

  if (!runNow)
    return {
      joinToken: null,
      reloadJoinToken: run,
      timedOut: false,
      timeout: 0,
    };

  if (existing) {
    if (existing.error) {
      throw existing.error;
    }

    if (existing.response) {
      return {
        joinToken: existing.response,
        reloadJoinToken() {
          // Delete the cached token and force a rerender
          // so that this hook runs again and creates a new one.

          joinTokenCache.delete(resourceKind);

          rerender(c => c + 1);
        },
        timedOut: tokenContext.timedOut,
        timeout: tokenContext.timeout,
      };
    }

    throw existing.promise;
  }

  throw run().promise;
}
