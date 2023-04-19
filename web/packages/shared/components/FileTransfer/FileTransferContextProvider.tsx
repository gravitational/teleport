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

import React, { useContext, useState, FC, createContext } from 'react';

import { FileTransferDialogDirection } from './FileTransferStateless';
import { FilesStore, useFilesStore } from './useFilesStore';

const FileTransferContext =
  createContext<{
    openedDialog: FileTransferDialogDirection;
    openDownloadDialog(): void;
    openUploadDialog(): void;
    closeDialog(): void;
    filesStore: FilesStore;
  }>(null);

export const FileTransferContextProvider: FC<{
  openedDialog?: FileTransferDialogDirection;
}> = props => {
  const filesStore = useFilesStore();
  const [openedDialog, setOpenedDialog] = useState<
    FileTransferDialogDirection | undefined
  >(props.openedDialog);

  function openDownloadDialog(): void {
    setOpenedDialog(FileTransferDialogDirection.Download);
  }

  function openUploadDialog(): void {
    setOpenedDialog(FileTransferDialogDirection.Upload);
  }

  function closeDialog(): void {
    setOpenedDialog(undefined);
  }

  return (
    <FileTransferContext.Provider
      value={{
        openedDialog,
        openDownloadDialog,
        openUploadDialog,
        closeDialog,
        filesStore,
      }}
      children={props.children}
    />
  );
};

export const useFileTransferContext = () => {
  const context = useContext(FileTransferContext);

  if (!context) {
    throw new Error(
      'FileTransfer requires FileTransferContextProvider context.'
    );
  }

  return context;
};
