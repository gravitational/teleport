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
import styled from 'styled-components';
import { ButtonIcon, Flex, Text } from 'design';
import { Close as CloseIcon } from 'design/Icon';

import { FileTransferDialogDirection, TransferredFile } from './types';
import { DownloadForm } from './DownloadForm';
import { UploadForm } from './UploadForm';
import { FileList } from './FileList';

export interface FileTransferStatelessProps {
  openedDialog: FileTransferDialogDirection;
  files: TransferredFile[];
  backgroundColor?: string;
  // errorText is any general error that isn't related to a specific transfer
  errorText?: string;

  onClose(): void;

  onAddDownload(sourcePath: string): void;

  onAddUpload(destinationPath: string, file: File): void;

  onCancel(id: string): void;
}

export function FileTransferStateless(props: FileTransferStatelessProps) {
  const items =
    props.openedDialog === FileTransferDialogDirection.Download
      ? {
          header: 'Download Files',
          Form: <DownloadForm onAddDownload={props.onAddDownload} />,
        }
      : {
          header: 'Upload Files',
          Form: <UploadForm onAddUpload={props.onAddUpload} />,
        };

  return (
    <Container
      data-testid="file-transfer-container"
      backgroundColor={props.backgroundColor}
      onKeyDown={e => {
        if (e.key !== 'Escape') {
          return;
        }

        e.preventDefault();
        e.stopPropagation();
        props.onClose();
      }}
    >
      <Flex justifyContent="space-between" alignItems="baseline">
        <Text fontSize={3} bold mb={3}>
          {items.header}
        </Text>
        <ButtonClose onClick={props.onClose} />
      </Flex>
      {items.Form}
      <Text color="error.light" typography="body2" mt={1}>
        {props.errorText}
      </Text>
      <FileList files={props.files} onCancel={props.onCancel} />
    </Container>
  );
}

function ButtonClose(props: { onClick(): void }) {
  return (
    <ButtonIcon title="Close" onClick={props.onClick}>
      <CloseIcon />
    </ButtonIcon>
  );
}

const Container = styled.div`
  background: ${props =>
    props.backgroundColor || props.theme.colors.levels.surface};
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
  box-sizing: border-box;
  border-radius: ${props => props.theme.radii[2]}px;
  padding: 8px 16px 16px;
  position: absolute;
  right: 8px;
  top: 8px;
  width: 500px;
  z-index: 10;
`;
