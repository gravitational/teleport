import { ClientReadableStream } from '@grpc/grpc-js';
import { FileTransferListeners } from 'shared/components/FileTransfer';

import { FileTransferProgress } from './v1/service_pb';
import * as api from './v1/service_pb';
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
