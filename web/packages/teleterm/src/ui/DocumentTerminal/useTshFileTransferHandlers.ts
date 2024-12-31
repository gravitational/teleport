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

import { FileTransferDirection } from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb';
import {
  createFileTransferEventsEmitter,
  FileTransferListeners,
} from 'shared/components/FileTransfer';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { IAppContext } from 'teleterm/ui/types';
import type * as uri from 'teleterm/ui/uri';
import { retryWithRelogin } from 'teleterm/ui/utils';

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
        FileTransferDirection.UPLOAD
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
        FileTransferDirection.DOWNLOAD
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
