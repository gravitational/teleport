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

import { useMemo, useState } from 'react';

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
  const [attempt, setState] = useState(() => ({
    ...defaultState,
    ...initialState,
  }));
  const actions = useMemo(() => makeActions(setState), [setState]);
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
