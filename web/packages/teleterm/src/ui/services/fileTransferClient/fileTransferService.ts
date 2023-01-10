import { FileTransferListeners } from 'shared/components/FileTransfer';

import { FileTransferRequest, TshClient } from 'teleterm/services/tshd/types';
import { UsageService } from 'teleterm/ui/services/usage';
import { FileTransferDirection } from 'teleterm/services/tshd/v1/service_pb';

export class FileTransferService {
  constructor(
    private tshClient: TshClient,
    private usageService: UsageService
  ) {}

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
    const listeners = this.tshClient.transferFile(options, abortSignal);
    if (
      options.direction ===
      FileTransferDirection.FILE_TRANSFER_DIRECTION_DOWNLOAD
    ) {
      this.usageService.captureFileTransferRun(options.clusterUri, {
        isUpload: false,
      });
    }
    if (
      options.direction === FileTransferDirection.FILE_TRANSFER_DIRECTION_UPLOAD
    ) {
      this.usageService.captureFileTransferRun(options.clusterUri, {
        isUpload: true,
      });
    }
    return listeners;
  }
}
