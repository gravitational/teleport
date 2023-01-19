import React, { useCallback, useContext, useEffect, useState } from 'react';

import { useTeleport } from 'teleport';

import {
  ResourceKind,
  resourceKindToJoinRole,
} from 'teleport/Discover/Shared/ResourceKind';

import type { AgentLabel } from 'teleport/services/agents';
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
  suggestedAgentMatcherLabels: AgentLabel[] = [],
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
          {
            roles: [resourceKindToJoinRole(resourceKind)],
            method: joinMethod,
            suggestedAgentMatcherLabels,
          },
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
