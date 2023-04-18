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

import { useState } from 'react';
import { FileTransferRequest } from 'shared/components/FileTransfer/FileTransferRequests';
import { useFilesStore } from 'shared/components/FileTransfer/useFilesStore';

import { UserContext } from 'teleport/services/user';

import { DocumentSsh } from '../stores';

import { getHttpFileTransferHandlers } from './httpFileTransferHandlers';
import useGetScpUrl from './useGetScpUrl';

export const useFileTransfer = ({ doc, user, addMfaToScpUrls }: Props) => {
  const filesStore = useFilesStore();
  const [fileTransferRequests, setFileTransferRequests] = useState<
    FileTransferRequest[]
  >([]);
  const { getScpUrl, attempt: getMfaResponseAttempt } =
    useGetScpUrl(addMfaToScpUrls);

  function updateFileTransferRequests(data: FileTransferRequest) {
    const newFileTransferRequest: FileTransferRequest = {
      ...data,
      isOwnRequest: user?.username === data.requester,
    };
    return setFileTransferRequests(prevstate => {
      // We receive the same data type when a file transfer request is created and
      // when an update event happens. Check if we already have this request in our list. If not
      // in our list, we add it
      const foundRequest = prevstate.find(
        ft => ft.requestID === newFileTransferRequest.requestID
      );
      if (!foundRequest) {
        return [...prevstate, newFileTransferRequest];
      } else {
        return prevstate.map(ft => {
          if (ft.requestID === newFileTransferRequest.requestID) {
            return newFileTransferRequest;
          }
          return ft;
        });
      }
    });
  }

  function handleFileTransferDenied(request: FileTransferRequest) {
    removeFileTransferRequest(request.requestID);
  }

  function handleFileTransferApproval(
    request: FileTransferRequest,
    file?: File
  ) {
    removeFileTransferRequest(request.requestID);
    if (request.requester !== user.username) {
      return;
    }

    if (request.download) {
      return filesStore.start({
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
    return filesStore.start({
      name: request.filename,
      runFileTransfer: abortController =>
        upload(request.location, file, abortController, {
          fileRequestId: request.requestID,
          moderatedSessionId: request.sid,
        }),
    });
  }

  function removeFileTransferRequest(requestId: string) {
    setFileTransferRequests(prevstate =>
      prevstate.filter(ft => ft.requestID !== requestId)
    );
  }

  async function download(
    location: string,
    abortController: AbortController,
    moderatedSessionParams?: ModeratedSessionParams
  ) {
    const url = await getScpUrl({
      location,
      clusterId: doc.clusterId,
      serverId: doc.serverId,
      login: doc.login,
      filename: location,
      moderatedSessonId: moderatedSessionParams?.moderatedSessionId,
      fileTransferRequestId: moderatedSessionParams?.fileRequestId,
    });
    if (!url) {
      // if we return nothing here, the file transfer will not be added to the
      // file transfer list. If we add it to the list, the file will continue to
      // start the download and return another here. This prevents a second network
      // request that we know will fail.
      return;
    }
    return getHttpFileTransferHandlers().download(url, abortController);
  }

  async function upload(
    location: string,
    file: File,
    abortController: AbortController,
    moderatedSessionParams?: ModeratedSessionParams
  ) {
    const url = await getScpUrl({
      location,
      clusterId: doc.clusterId,
      serverId: doc.serverId,
      login: doc.login,
      filename: file.name,
      moderatedSessonId: moderatedSessionParams?.moderatedSessionId,
      fileTransferRequestId: moderatedSessionParams?.fileRequestId,
    });
    if (!url) {
      // if we return nothing here, the file transfer will not be added to the
      // file transfer list. If we add it to the list, the file will continue to
      // start the download and return another here. This prevents a second network
      // request that we know will fail.
      return;
    }
    return getHttpFileTransferHandlers().upload(url, file, abortController);
  }

  return {
    download,
    upload,
    fileTransferRequests,
    getMfaResponseAttempt,
    updateFileTransferRequests,
    handleFileTransferApproval,
    handleFileTransferDenied,
    filesStore,
  };
};

type Props = {
  user: UserContext;
  addMfaToScpUrls: boolean;
  doc: DocumentSsh;
};

type ModeratedSessionParams = {
  fileRequestId: string;
  moderatedSessionId: string;
};
