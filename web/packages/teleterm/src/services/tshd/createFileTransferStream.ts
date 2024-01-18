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

import { ClientReadableStream } from '@grpc/grpc-js';
import { FileTransferListeners } from 'shared/components/FileTransfer';
import { FileTransferProgress } from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb';
import * as api from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb';

import { TshAbortSignal } from './types';

export function createFileTransferStream(
  stream: ClientReadableStream<FileTransferProgress>,
  abortSignal: TshAbortSignal
): FileTransferListeners {
  abortSignal.addEventListener(() => stream.cancel());

  return {
    onProgress(callback: (progress: number) => void) {
      stream.on('data', (data: api.FileTransferProgress) =>
        callback(data.percentage)
      );
    },
    onComplete(callback: () => void) {
      stream.on('end', () => {
        callback();
        // When stream ends, all listeners can be removed.
        stream.removeAllListeners();
      });
    },
    onError(callback: (error: Error) => void) {
      stream.on('error', err => {
        callback(err);
        // Due to a bug in grpc-js, the `error` event is also emitted after the `end` event.
        // This behavior is not correct, only one of them should be emitted.
        // To fix this, we remove all listeners after the stream ended with an error.
        stream.removeAllListeners();
      });
    },
  };
}
