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
