import { FileTransferListeners } from 'shared/components/FileTransfer';

import { TshClient, FileTransferRequest } from 'teleterm/services/tshd/types';

export class FileTransferService {
  constructor(private tshClient: TshClient) {}

  transferFile(
    options: FileTransferRequest,
    abortController: AbortController
  ): FileTransferListeners {
    const abortSignal = {
      addEventListener: (cb: (...args: any[]) => void) => {
        abortController.signal.addEventListener('abort', cb);
      },
      removeEventListener: (cb: (...args: any[]) => void) => {
        abortController.signal.removeEventListener('abort', cb);
      },
    };
    return this.tshClient.transferFile(options, abortSignal);
  }
}
