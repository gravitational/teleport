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
