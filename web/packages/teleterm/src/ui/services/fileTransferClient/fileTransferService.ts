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

import { FileTransferListeners } from 'shared/components/FileTransfer';

import { FileTransferDirection } from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb';

import { FileTransferRequest } from 'teleterm/services/tshd/types';
import { TshdClient } from 'teleterm/services/tshd';
import { UsageService } from 'teleterm/ui/services/usage';
import { cloneAbortSignal } from 'teleterm/services/tshd/cloneableClient';

export class FileTransferService {
  constructor(
    private tshClient: TshdClient,
    private usageService: UsageService
  ) {}

  transferFile(
    request: FileTransferRequest,
    abortController: AbortController
  ): FileTransferListeners {
    const stream = this.tshClient.transferFile(request, {
      abort: cloneAbortSignal(abortController.signal),
    });
    if (request.direction === FileTransferDirection.DOWNLOAD) {
      this.usageService.captureFileTransferRun(request.serverUri, {
        isUpload: false,
      });
    }
    if (request.direction === FileTransferDirection.UPLOAD) {
      this.usageService.captureFileTransferRun(request.serverUri, {
        isUpload: true,
      });
    }
    return {
      onProgress(callback: (progress: number) => void) {
        stream.responses.onMessage(data => callback(data.percentage));
      },
      onComplete: stream.responses.onComplete,
      onError: stream.responses.onError,
    };
  }
}
