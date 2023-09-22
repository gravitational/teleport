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

import { FileTransferListeners } from 'shared/components/FileTransfer';

import {
  FileTransferDirection,
  FileTransferRequest,
  TshClient,
} from 'teleterm/services/tshd/types';
import { UsageService } from 'teleterm/ui/services/usage';

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
      this.usageService.captureFileTransferRun(options.serverUri, {
        isUpload: false,
      });
    }
    if (
      options.direction === FileTransferDirection.FILE_TRANSFER_DIRECTION_UPLOAD
    ) {
      this.usageService.captureFileTransferRun(options.serverUri, {
        isUpload: true,
      });
    }
    return listeners;
  }
}
