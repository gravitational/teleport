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

import React from 'react';

import Logger from 'shared/libs/logger';
const logger = Logger.create('shared/hooks/useAttempt');

const defaultState = {
  isProcessing: false,
  isFailed: false,
  isSuccess: false,
  message: '',
};

export default function useAttempt(
  initialState: Partial<State>
): [State, Actions] {
  const [attempt, setState] = React.useState(() => ({
    ...defaultState,
    ...initialState,
  }));
  const actions = React.useMemo(() => makeActions(setState), [setState]);
  return [attempt, actions];
}

function makeActions(setState) {
  function stop(message = '') {
    setState({ ...defaultState, isSuccess: true, message });
  }

  function start() {
    setState({ ...defaultState, isProcessing: true });
  }

  function clear() {
    setState({ ...defaultState });
  }

  function error(err: Error) {
    logger.error('attempt', err);
    setState({ ...defaultState, isFailed: true, message: err.message });
  }

  function run(fn: Callback) {
    try {
      start();
      return fn()
        .then(() => {
          stop();
        })
        .catch(err => {
          error(err);
          throw err;
        });
    } catch (err) {
      error(err);
    }
  }

  return {
    do: run,
    stop,
    start,
    clear,
    error,
  };
}

type State = typeof defaultState;

type Actions = {
  do: (fn: Callback) => Promise<any>;
  stop: (message?: string) => void;
  start: () => void;
  clear: () => void;
  error: (err: Error) => void;
};

type Callback = (fn?: any) => Promise<any>;
