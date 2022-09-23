import React, { useCallback, useContext, useEffect, useState } from 'react';

import { JoinToken } from 'teleport/services/joinToken';
import { useTeleport } from 'teleport';

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
let result: PromiseResult;

export function useJoinTokenValue() {
  const tokenContext = useContext(joinTokenContext);

  return tokenContext.joinToken;
}

export function useJoinToken(): {
  joinToken: JoinToken;
  reloadJoinToken: () => void;
  timedOut: boolean;
  timeout: number;
} {
  const ctx = useTeleport();
  const tokenContext = useContext(joinTokenContext);

  function run() {
    abortController = new AbortController();

    result = {
      promise: ctx.joinTokenService
        .fetchJoinToken(['WindowsDesktop'], 'token', [], abortController.signal)
        .then(token => {
          // Probably will never happen, but just in case, otherwise
          // querying for the resource can return a false positive.
          if (!token.internalResourceId) {
            throw new Error(
              'internal resource ID is required to discover the newly added resource, but none was provided'
            );
          }

          return token;
        })
        .then(response => {
          result.response = response;

          tokenContext.setJoinToken(response);
          tokenContext.startTimer();
        })
        .catch(error => (result.error = error)),
    };

    return result;
  }

  useEffect(() => {
    return () => abortController.abort();
  }, []);

  if (result) {
    if (result.error) {
      throw result.error;
    }

    if (result.response) {
      return {
        joinToken: result.response,
        reloadJoinToken: run,
        timedOut: tokenContext.timedOut,
        timeout: tokenContext.timeout,
      };
    }

    throw result.promise;
  }

  throw run().promise;
}
