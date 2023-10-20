/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { FileTransferContainer } from '../FileTransferContainer';

import {
  FileTransferStateless,
  FileTransferStatelessProps,
} from './FileTransferStateless';
import { FileTransferDialogDirection, TransferredFile } from './types';

export default {
  title: 'Shared/FileTransfer',
};

const defaultFiles: TransferredFile[] = [
  {
    id: '1547581437406~/test',
    name: '~/Users/grzegorz/Makefile',
    transferState: {
      type: 'processing',
      progress: 10,
    },
  },
  {
    id: '1547581437406~/test',
    name: '~Users/grzegorz/very/long/path/that/does/not/exist/but/is/very/useful/for/storybook/stories',
    transferState: {
      type: 'processing',
      progress: 64,
    },
  },
];

function GetFileTransfer(
  props: Pick<FileTransferStatelessProps, 'openedDialog' | 'files'>
) {
  return (
    <FileTransferContainer>
      <FileTransferStateless
        openedDialog={props.openedDialog}
        files={props.files}
        onClose={() => undefined}
        onAddDownload={() => undefined}
        onAddUpload={() => undefined}
        onCancel={() => undefined}
      />
    </FileTransferContainer>
  );
}

export const DownloadProgress = () => (
  <GetFileTransfer
    files={defaultFiles}
    openedDialog={FileTransferDialogDirection.Download}
  ></GetFileTransfer>
);

export const DownloadError = () => (
  <GetFileTransfer
    files={[
      {
        ...defaultFiles[0],
        transferState: {
          type: 'error',
          progress: 11,
          error: new Error(
            'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.'
          ),
        },
      },
      {
        ...defaultFiles[1],
        transferState: {
          type: 'error',
          progress: 0,
          error: new Error('stat /root/test: no such file or directory'),
        },
      },
    ]}
    openedDialog={FileTransferDialogDirection.Download}
  ></GetFileTransfer>
);

export const DownloadCompleted = () => (
  <GetFileTransfer
    files={[
      {
        ...defaultFiles[0],
        transferState: {
          type: 'completed',
        },
      },
      {
        ...defaultFiles[1],
        transferState: {
          type: 'completed',
        },
      },
    ]}
    openedDialog={FileTransferDialogDirection.Download}
  ></GetFileTransfer>
);

export const DownloadLongList = () => (
  <GetFileTransfer
    files={[
      defaultFiles[0],
      defaultFiles[0],
      defaultFiles[0],
      defaultFiles[0],
      defaultFiles[0],
      defaultFiles[0],
      defaultFiles[0],
      defaultFiles[0],
      defaultFiles[0],
    ]}
    openedDialog={FileTransferDialogDirection.Download}
  ></GetFileTransfer>
);

export const UploadProgress = () => (
  <GetFileTransfer
    files={defaultFiles}
    openedDialog={FileTransferDialogDirection.Upload}
  ></GetFileTransfer>
);

export const UploadError = () => (
  <GetFileTransfer
    files={[
      {
        ...defaultFiles[0],
        transferState: {
          type: 'error',
          progress: 11,
          error: new Error(
            'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.'
          ),
        },
      },
      {
        ...defaultFiles[1],
        transferState: {
          type: 'error',
          progress: 0,
          error: new Error('stat /root/test: no such file or directory'),
        },
      },
    ]}
    openedDialog={FileTransferDialogDirection.Upload}
  ></GetFileTransfer>
);

export const UploadCompleted = () => (
  <GetFileTransfer
    files={[
      {
        ...defaultFiles[0],
        transferState: {
          type: 'completed',
        },
      },
      {
        ...defaultFiles[1],
        transferState: {
          type: 'completed',
        },
      },
    ]}
    openedDialog={FileTransferDialogDirection.Upload}
  ></GetFileTransfer>
);
