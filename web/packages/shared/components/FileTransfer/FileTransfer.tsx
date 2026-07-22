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

import type { JSX } from 'react';

import { FileTransferContainer } from './FileTransferContainer';
import { useFileTransferContext } from './FileTransferContextProvider';
import {
  FileTransferDialogDirection,
  FileTransferListeners,
  FileTransferStateless,
} from './FileTransferStateless';

interface FileTransferProps {
  transferHandlers: TransferHandlers;
  // errorText is any general error that isn't related to a specific transfer
  errorText?: string;
  /**
   * `beforeClose` is called when an attempt to close the dialog was made
   * and there is a file transfer in progress.
   * Returning `true` will close the dialog, returning `false` will not.
   */
  beforeClose?(): Promise<boolean> | boolean;

  afterClose?(): void;

  FileTransferRequestsComponent?: JSX.Element;
}

/**
 * Both `getDownloader` and `getUploader` can return a promise containing `FileTransferListeners` function or nothing.
 * In the latter case, the file will not be added to the list and the download will not start.
 */
export interface TransferHandlers {
  getDownloader: (
    sourcePath: string,
    abortController: AbortController
  ) => Promise<FileTransferListeners | undefined>;
  getUploader: (
    destinationPath: string,
    file: File,
    abortController: AbortController
  ) => Promise<FileTransferListeners | undefined>;
}

export function FileTransfer(props: FileTransferProps) {
  const { openedDialog, closeDialog } = useFileTransferContext();

  async function handleCloseDialog(
    isAnyTransferInProgress: boolean
  ): Promise<void> {
    const runCloseCallbacks = () => {
      closeDialog();
      props.afterClose?.();
    };

    if (!isAnyTransferInProgress || !props.beforeClose) {
      runCloseCallbacks();
      return;
    }

    if (await props.beforeClose()) {
      runCloseCallbacks();
    }
  }

  return (
    <FileTransferContainer>
      {props.FileTransferRequestsComponent}
      {openedDialog && (
        <FileTransferDialog
          errorText={props.errorText}
          openedDialog={openedDialog}
          transferHandlers={props.transferHandlers}
          onCloseDialog={handleCloseDialog}
        />
      )}
    </FileTransferContainer>
  );
}

export function FileTransferDialog(
  props: Pick<FileTransferProps, 'transferHandlers' | 'errorText'> & {
    openedDialog: FileTransferDialogDirection;
    onCloseDialog(isAnyTransferInProgress: boolean): void;
  }
) {
  const { filesStore } = useFileTransferContext();

  function handleAddDownload(sourcePath: string): void {
    filesStore.start({
      name: sourcePath,
      runFileTransfer: abortController =>
        props.transferHandlers.getDownloader(sourcePath, abortController),
    });
  }

  function handleAddUpload(destinationPath: string, file: File): void {
    filesStore.start({
      name: file.name,
      runFileTransfer: abortController =>
        props.transferHandlers.getUploader(
          destinationPath,
          file,
          abortController
        ),
    });
  }

  function handleClose(): void {
    props.onCloseDialog(filesStore.isAnyTransferInProgress());
  }

  return (
    <FileTransferStateless
      errorText={props.errorText}
      openedDialog={props.openedDialog}
      files={filesStore.files}
      onCancel={filesStore.cancel}
      onClose={handleClose}
      onAddUpload={handleAddUpload}
      onAddDownload={handleAddDownload}
    />
  );
}
