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

interface PromiseResult {
  promise?: Promise<any>;
  response?: JoinToken;
  error?: Error;
}

let abortController: AbortController;
let cachedJoinTokenResult: PromiseResult;

export function clearCachedJoinTokenResult() {
  cachedJoinTokenResult = null;
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

  function run() {
    abortController = new AbortController();

    cachedJoinTokenResult = {
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
          cachedJoinTokenResult.response = token;
          tokenContext.setJoinToken(token);
          tokenContext.startTimer();
        })
        .catch(error => {
          cachedJoinTokenResult.error = error;
        }),
    };

    return cachedJoinTokenResult;
  }

  useEffect(() => {
    return () => {
      abortController?.abort();

      // result will be stored in memory which can refer to
      // previously used or expired join tokens.
      clearCachedJoinTokenResult();
    };
  }, []);

  if (!runNow)
    return {
      joinToken: null,
      reloadJoinToken: run,
      timedOut: false,
      timeout: 0,
    };

  if (cachedJoinTokenResult) {
    if (cachedJoinTokenResult.error) {
      throw cachedJoinTokenResult.error;
    }

    if (cachedJoinTokenResult.response) {
      return {
        joinToken: cachedJoinTokenResult.response,
        reloadJoinToken: run,
        timedOut: tokenContext.timedOut,
        timeout: tokenContext.timeout,
      };
    }

    throw cachedJoinTokenResult.promise;
  }

  throw run().promise;
}
