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

import { ClientReadableStream } from '@grpc/grpc-js';
import { FileTransferListeners } from 'shared/components/FileTransfer';
import { FileTransferProgress } from 'gen-proto-js/teleport/lib/teleterm/v1/service_pb';
import * as api from 'gen-proto-js/teleport/lib/teleterm/v1/service_pb';

import { TshAbortSignal } from './types';

export function createFileTransferStream(
  stream: ClientReadableStream<FileTransferProgress>,
  abortSignal?: TshAbortSignal
): FileTransferListeners {
  abortSignal.addEventListener(() => stream.cancel());

  return {
    onProgress(callback: (progress: number) => void) {
      stream.on('data', (data: api.FileTransferProgress) =>
        callback(data.getPercentage())
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
