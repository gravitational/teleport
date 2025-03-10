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

import styled from 'styled-components';

import { ButtonIcon, Flex, Text } from 'design';
import { Cross as CloseIcon } from 'design/Icon';

import { DownloadForm } from './DownloadForm';
import { FileList } from './FileList';
import { FileTransferDialogDirection, TransferredFile } from './types';
import { UploadForm } from './UploadForm';

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
      {/* TODO(bl-nero): This should be a part of the new input design (in the helper text line). */}
      <Text color="error.hover" typography="body3" mt={1}>
        {props.errorText}
      </Text>
      <FileList files={props.files} onCancel={props.onCancel} />
    </Container>
  );
}

function ButtonClose(props: { onClick(): void }) {
  return (
    <ButtonIcon title="Close" onClick={props.onClick}>
      <CloseIcon size="medium" />
    </ButtonIcon>
  );
}

const Container = styled.div`
  background: ${props => props.theme.colors.levels.surface};
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
  box-sizing: border-box;
  border-radius: ${props => props.theme.radii[2]}px;
  padding: 8px 16px 16px;
`;
