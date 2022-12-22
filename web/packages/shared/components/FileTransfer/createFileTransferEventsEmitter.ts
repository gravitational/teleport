/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { EventEmitter } from 'events';

import { FileTransferListeners } from './FileTransferStateless/types';

export interface FileTransferEventsEmitter extends FileTransferListeners {
  emitProgress(progress: number): void;

  emitError(error: Error): void;

  emitComplete(): void;
}

/**
 * `createFileTransferEventsEmitter` is a utility function that helps with
 * generating events that can be consumed by a function expecting `FileTransferListeners`.
 */
export function createFileTransferEventsEmitter(): FileTransferEventsEmitter {
  const events = new EventEmitter();
  return {
    emitProgress: progress => {
      events.emit('progress', progress);
    },
    emitComplete: () => {
      events.emit('complete');
    },
    emitError: error => {
      events.emit('error', error);
    },
    onProgress: callback => {
      events.on('progress', callback);
    },
    onComplete: callback => {
      events.on('complete', callback);
    },
    onError: callback => {
      events.on('error', callback);
    },
  };
}
