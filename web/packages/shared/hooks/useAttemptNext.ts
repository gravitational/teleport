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

import { useCallback, useState } from 'react';

import Logger from 'shared/libs/logger';

const logger = Logger.create('shared/hooks/useAttempt');

// This is the next version of existing useAttempt hook
export default function useAttemptNext(status = '' as Attempt['status']) {
  const [attempt, setAttempt] = useState<Attempt>(() => ({
    status,
    statusText: '',
  }));

  const handleError = useCallback((err: Error) => {
    logger.error('attempt', err);
    setAttempt({ status: 'failed', statusText: err.message });
  }, []);

  const run = useCallback((fn: Callback) => {
    try {
      setAttempt({ status: 'processing' });
      return fn()
        .then(() => {
          setAttempt({ status: 'success' });
          return true;
        })
        .catch(err => {
          handleError(err);
          return false;
        });
    } catch (err) {
      handleError(err);
      return Promise.resolve(false);
    }
  }, []);

  return { attempt, setAttempt, run, handleError };
}

export type Attempt = {
  status: 'processing' | 'failed' | 'success' | '';
  statusText?: string;
  statusCode?: number;
};

type Callback = (fn?: any) => Promise<any>;

export type State = ReturnType<typeof useAttemptNext>;
