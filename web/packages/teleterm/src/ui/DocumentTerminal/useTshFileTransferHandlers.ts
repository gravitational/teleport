import {
  FileTransferListeners,
  createFileTransferEventsEmitter,
} from 'shared/components/FileTransfer';

import { routing } from 'teleterm/ui/uri';
import { FileTransferDirection } from 'teleterm/services/tshd/v1/service_pb';
import { retryWithRelogin } from 'teleterm/ui/utils';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { IAppContext } from 'teleterm/ui/types';

export function useTshFileTransferHandlers(options: {
  originatingDocumentUri: string;
}) {
  const appContext = useAppContext();

  return {
    upload(
      file: FileTransferRequestObject,
      abortController: AbortController
    ): FileTransferListeners {
      return transferFile(
        appContext,
        options.originatingDocumentUri,
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
        options.originatingDocumentUri,
        file,
        abortController,
        FileTransferDirection.FILE_TRANSFER_DIRECTION_DOWNLOAD
      );
    },
  };
}

function transferFile(
  appContext: IAppContext,
  originatingDocumentUri: string,
  file: FileTransferRequestObject,
  abortController: AbortController,
  direction: FileTransferDirection
): FileTransferListeners {
  const server = appContext.clustersService.getServer(file.serverUri);
  const eventsEmitter = createFileTransferEventsEmitter();
  const getFileTransferActionAsPromise = () =>
    new Promise((resolve, reject) => {
      const callbacks = appContext.fileTransferService.transferFile(
        {
          source: file.source,
          destination: file.destination,
          login: file.login,
          clusterUri: routing.ensureClusterUri(file.serverUri),
          hostname: server.hostname,
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

  retryWithRelogin(
    appContext,
    originatingDocumentUri,
    file.serverUri,
    getFileTransferActionAsPromise
  )
    .then(eventsEmitter.emitComplete)
    .catch(eventsEmitter.emitError);

  return eventsEmitter;
}

type FileTransferRequestObject = {
  source: string;
  destination: string;
  login: string;
  serverUri: string;
};
