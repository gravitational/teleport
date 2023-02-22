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
import { Flex, ButtonIcon } from 'design';
import * as Icons from 'design/Icon';

import { useFileTransferContext } from './FileTransferContextProvider';

type FileTransferActionBarProps = {
  isConnected: boolean;
};

export function FileTransferActionBar({
  isConnected,
}: FileTransferActionBarProps) {
  const fileTransferContext = useFileTransferContext();
  const areFileTransferButtonsDisabled =
    !!fileTransferContext.openedDialog || !isConnected;

  return (
    <FileTransferActionBarStateless
      disabled={areFileTransferButtonsDisabled}
      openDownloadDialog={fileTransferContext.openDownloadDialog}
      openUploadDialog={fileTransferContext.openUploadDialog}
    />
  );
}

export function FileTransferActionBarStateless({
  disabled,
  openDownloadDialog,
  openUploadDialog,
}: {
  disabled: boolean;
  openDownloadDialog: () => void;
  openUploadDialog: () => void;
}) {
  return (
    <Flex flex="none" alignItems="center" height="24px">
      <ButtonIcon
        disabled={disabled}
        size={0}
        title="Download files"
        onClick={openDownloadDialog}
      >
        <Icons.Download fontSize="16px" />
      </ButtonIcon>
      <ButtonIcon
        disabled={disabled}
        size={0}
        title="Upload files"
        onClick={openUploadDialog}
      >
        <Icons.Upload fontSize="16px" />
      </ButtonIcon>
    </Flex>
  );
}
