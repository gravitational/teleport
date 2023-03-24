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

import { EventEmitter } from 'events';

import { TshAbortController } from './types';

/**
 * Creates a version of AbortController that can be passed through Electron contextBridge
 */
export default function createAbortController(): TshAbortController {
  const emitter = new EventEmitter();

  const signal = {
    addEventListener(cb: (...args: any[]) => void) {
      emitter.addListener('abort', cb);
    },

    removeEventListener(cb: (...args: any[]) => void) {
      emitter.removeListener('abort', cb);
    },
  };

  return {
    signal,
    abort() {
      emitter.emit('abort');
    },
  };
}
