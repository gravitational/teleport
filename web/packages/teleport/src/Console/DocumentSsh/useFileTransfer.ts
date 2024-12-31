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

import { useCallback, useEffect, useState } from 'react';

import { useFileTransferContext } from 'shared/components/FileTransfer';

import cfg from 'teleport/config';
import { DocumentSsh } from 'teleport/Console/stores';
import { EventType } from 'teleport/lib/term/enums';
import Tty from 'teleport/lib/term/tty';
import { MfaState } from 'teleport/lib/useMfa';
import { Session } from 'teleport/services/session';

import { useConsoleContext } from '../consoleContextProvider';
import { getHttpFileTransferHandlers } from './httpFileTransferHandlers';

export type FileTransferRequest = {
  sid: string;
  requestID: string;
  requester: string;
  /** A list of accounts that approved this request. */
  approvers: string[];
  location: string;
  filename?: string;
  download: boolean;
};

export const isOwnRequest = (
  request: FileTransferRequest,
  currentUser: string
) => {
  return request.requester === currentUser;
};

export const useFileTransfer = (
  tty: Tty,
  session: Session,
  currentDoc: DocumentSsh,
  mfa: MfaState
) => {
  const { filesStore } = useFileTransferContext();
  const startTransfer = filesStore.start;
  const ctx = useConsoleContext();
  const currentUser = ctx.getStoreUser();
  const [fileTransferRequests, setFileTransferRequests] = useState<
    FileTransferRequest[]
  >([]);
  const { clusterId, serverId, login } = currentDoc;

  const download = useCallback(
    async (
      location: string,
      abortController: AbortController,
      moderatedSessionParams?: ModeratedSessionParams
    ) => {
      const mfaResponse = await mfa.getChallengeResponse();
      const url = cfg.getScpUrl({
        location,
        clusterId,
        serverId,
        login,
        filename: location,
        moderatedSessionId: moderatedSessionParams?.moderatedSessionId,
        fileTransferRequestId: moderatedSessionParams?.fileRequestId,
        mfaResponse,
      });

      if (!url) {
        // if we return nothing here, the file transfer will not be added to the
        // file transfer list. If we add it to the list, the file will continue to
        // start the download and return another here. This prevents a second network
        // request that we know will fail.
        return;
      }
      return getHttpFileTransferHandlers().download(url, abortController);
    },
    [clusterId, login, serverId, mfa]
  );

  const upload = useCallback(
    async (
      location: string,
      file: File,
      abortController: AbortController,
      moderatedSessionParams?: ModeratedSessionParams
    ) => {
      const mfaResponse = await mfa.getChallengeResponse();

      const url = cfg.getScpUrl({
        location,
        clusterId,
        serverId,
        login,
        filename: file.name,
        moderatedSessionId: moderatedSessionParams?.moderatedSessionId,
        fileTransferRequestId: moderatedSessionParams?.fileRequestId,
        mfaResponse,
      });
      if (!url) {
        // if we return nothing here, the file transfer will not be added to the
        // file transfer list. If we add it to the list, the file will continue to
        // start the download and return another here. This prevents a second network
        // request that we know will fail.
        return;
      }
      return getHttpFileTransferHandlers().upload(url, file, abortController);
    },
    [clusterId, serverId, login, mfa]
  );

  /*
   * TTY event listeners
   */

  // handleFileTransferDenied is called when a FILE_TRANSFER_REQUEST_DENY event is received
  // from the tty.
  const handleFileTransferDenied = useCallback(
    (request: FileTransferRequest) => {
      removeFileTransferRequest(request.requestID);
    },
    []
  );

  // handleFileTransferApproval is called when a FILE_TRANSFER_REQUEST_APPROVE event is received.
  // This isn't called when a single approval is received, but rather when the request approval policy has been
  // completely fulfilled, i.e. "This request requires two moderators approval and we received both". Any approve that
  // doesn't fulfill the policy will be sent as an update and handled in handleFileTransferUpdate
  const handleFileTransferApproval = useCallback(
    (request: FileTransferRequest, file?: File) => {
      removeFileTransferRequest(request.requestID);
      if (!isOwnRequest(request, currentUser.username)) {
        return;
      }

      if (request.download) {
        return startTransfer({
          name: request.location,
          runFileTransfer: abortController =>
            download(request.location, abortController, {
              fileRequestId: request.requestID,
              moderatedSessionId: request.sid,
            }),
        });
      }

      // if it gets here, it's an upload
      if (!file) {
        throw new Error('Approved file not found for upload.');
      }
      return startTransfer({
        name: request.filename,
        runFileTransfer: abortController =>
          upload(request.location, file, abortController, {
            fileRequestId: request.requestID,
            moderatedSessionId: request.sid,
          }),
      });
    },
    [currentUser.username, download, startTransfer, upload]
  );

  // handleFileTransferUpdate is called when a FILE_TRANSFER_REQUEST event is received. This is used when
  // we receive a new file transfer request, or when a request has been updated with an approval but its policy isn't
  // completely approved yet. An update in this way generally means that the approver array is updated.
  function handleFileTransferUpdate(data: FileTransferRequest) {
    setFileTransferRequests(prevstate => {
      // We receive the same data type when a file transfer request is created and
      // when an update event happens. Check if we already have this request in our list. If not
      // in our list, we add it
      const foundRequest = prevstate.find(
        ft => ft.requestID === data.requestID
      );
      if (!foundRequest) {
        return [...prevstate, data];
      } else {
        return prevstate.map(ft => {
          if (ft.requestID === data.requestID) {
            return data;
          }
          return ft;
        });
      }
    });
  }

  useEffect(() => {
    // the tty will be init outside of this hook, so we wait until
    // it exists and then attach file transfer handlers to it
    if (!tty) {
      return;
    }
    tty.on(EventType.FILE_TRANSFER_REQUEST, handleFileTransferUpdate);
    tty.on(EventType.FILE_TRANSFER_REQUEST_APPROVE, handleFileTransferApproval);
    tty.on(EventType.FILE_TRANSFER_REQUEST_DENY, handleFileTransferDenied);
    return () => {
      tty.removeListener(
        EventType.FILE_TRANSFER_REQUEST,
        handleFileTransferUpdate
      );
      tty.removeListener(
        EventType.FILE_TRANSFER_REQUEST_APPROVE,
        handleFileTransferApproval
      );
      tty.removeListener(
        EventType.FILE_TRANSFER_REQUEST_DENY,
        handleFileTransferDenied
      );
    };
  }, [tty, handleFileTransferDenied, handleFileTransferApproval]);

  function removeFileTransferRequest(requestId: string) {
    setFileTransferRequests(prevstate =>
      prevstate.filter(ft => ft.requestID !== requestId)
    );
  }

  /*
   * Transfer handlers
   */

  async function getDownloader(
    location: string,
    abortController: AbortController
  ) {
    if (session.moderated) {
      tty.sendFileDownloadRequest(location);
      return;
    }

    return download(location, abortController);
  }

  async function getUploader(
    location: string,
    file: File,
    abortController: AbortController
  ) {
    if (session.moderated) {
      tty.sendFileUploadRequest(location, file);
      return;
    }

    return upload(location, file, abortController);
  }

  return {
    fileTransferRequests,
    getUploader,
    getDownloader,
  };
};

type ModeratedSessionParams = {
  fileRequestId: string;
  moderatedSessionId: string;
};
