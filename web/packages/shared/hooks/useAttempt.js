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

import { useState } from 'react';
import Logger from 'shared/libs/logger';

const logger = Logger.create('shared/hooks/useState');

const defaultAttempt = {
  isProcessing: false,
  isFailed: false,
  isSuccess: false,
  message: '',
};

export default function useAttempt(initialState) {
  initialState = initialState || defaultAttempt;
  const [attempt, change] = useState(initialState);

  const actions = {
    do(fn) {
      try {
        this.start();
        return fn()
          .then(() => {
            this.stop();
          })
          .catch(err => {
            this.error(err);
            throw err;
          });
      } catch (err) {
        this.error(err);
      }
    },

    stop(message) {
      change({ ...defaultAttempt, isSuccess: true, message });
    },

    start() {
      change({ ...defaultAttempt, isProcessing: true });
    },

    clear() {
      change({ ...defaultAttempt });
    },

    error(err) {
      logger.error('attempt', err);
      change({ ...defaultAttempt, isFailed: true, message: err.message });
    },
  };

  return [attempt, actions];
}
