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

import {
  FileTransferListeners,
  createFileTransferEventsEmitter,
} from 'shared/components/FileTransfer';

import { FileTransferDirection } from 'teleterm/services/tshd/types';
import { retryWithRelogin } from 'teleterm/ui/utils';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { IAppContext } from 'teleterm/ui/types';

import type * as uri from 'teleterm/ui/uri';

export function useTshFileTransferHandlers() {
  const appContext = useAppContext();

  return {
    upload(
      file: FileTransferRequestObject,
      abortController: AbortController
    ): FileTransferListeners {
      return transferFile(
        appContext,
        file,
        abortController,
        FileTransferDirection.FILE_TRANSFER_DIRECTION_UPLOAD
      );
    },
    download(
      file: FileTransferRequestObject,
      abortController: AbortController
    ): FileTransferListeners {
      return transferFile(
        appContext,
        file,
        abortController,
        FileTransferDirection.FILE_TRANSFER_DIRECTION_DOWNLOAD
      );
    },
  };
}

function transferFile(
  appContext: IAppContext,
  file: FileTransferRequestObject,
  abortController: AbortController,
  direction: FileTransferDirection
): FileTransferListeners {
  const eventsEmitter = createFileTransferEventsEmitter();
  const getFileTransferActionAsPromise = () =>
    new Promise((resolve, reject) => {
      const callbacks = appContext.fileTransferService.transferFile(
        {
          serverUri: file.serverUri,
          source: file.source,
          destination: file.destination,
          login: file.login,
          direction,
        },
        abortController
      );

      callbacks.onProgress((percentage: number) => {
        eventsEmitter.emitProgress(percentage);
      });
      callbacks.onError((error: Error) => {
        reject(error);
      });
      callbacks.onComplete(() => {
        resolve(undefined);
      });
    });

  retryWithRelogin(appContext, file.serverUri, getFileTransferActionAsPromise)
    .then(eventsEmitter.emitComplete)
    .catch(eventsEmitter.emitError);

  return eventsEmitter;
}

type FileTransferRequestObject = {
  source: string;
  destination: string;
  login: string;
  serverUri: uri.ServerUri;
};
