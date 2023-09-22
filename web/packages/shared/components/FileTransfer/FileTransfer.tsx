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

import { useFileTransferContext } from './FileTransferContextProvider';
import { useFilesStore } from './useFilesStore';
import {
  FileTransferDialogDirection,
  FileTransferListeners,
  FileTransferStateless,
} from './FileTransferStateless';

interface FileTransferProps {
  backgroundColor?: string;
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

  if (!openedDialog) {
    return null;
  }

  return (
    <FileTransferDialog
      errorText={props.errorText}
      openedDialog={openedDialog}
      backgroundColor={props.backgroundColor}
      transferHandlers={props.transferHandlers}
      onCloseDialog={handleCloseDialog}
    />
  );
}

export function FileTransferDialog(
  props: Pick<
    FileTransferProps,
    'transferHandlers' | 'backgroundColor' | 'errorText'
  > & {
    openedDialog: FileTransferDialogDirection;
    onCloseDialog(isAnyTransferInProgress: boolean): void;
  }
) {
  const filesStore = useFilesStore();

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
      backgroundColor={props.backgroundColor}
      onClose={handleClose}
      onAddUpload={handleAddUpload}
      onAddDownload={handleAddDownload}
    />
  );
}
