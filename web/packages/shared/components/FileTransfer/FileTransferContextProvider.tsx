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

import {
  createContext,
  FC,
  PropsWithChildren,
  useContext,
  useState,
} from 'react';

import { FileTransferDialogDirection } from './FileTransferStateless';
import { FilesStore, useFilesStore } from './useFilesStore';

const FileTransferContext = createContext<{
  openedDialog: FileTransferDialogDirection;
  openDownloadDialog(): void;
  openUploadDialog(): void;
  closeDialog(): void;
  filesStore: FilesStore;
}>(null);

export const FileTransferContextProvider: FC<
  PropsWithChildren<{
    openedDialog?: FileTransferDialogDirection;
  }>
> = props => {
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
