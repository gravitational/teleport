/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import styled from 'styled-components';
import { colors } from 'teleport/console/components/colors';
import DownloadForm from './DownloadForm';
import UploadForm from './UploadForm';
import FileList from './FileList';
import { ButtonClose } from './Elements';
import useScpContext, { ScpContextProvider } from './scpContextProvider';
import { Scp } from './scpContext';

export default function FileTransferDialogs(props: FileTransferDialogsProps) {
  const {
    isDownloadOpen,
    isUploadOpen,
    onClose,
    clusterId,
    serverId,
    login,
  } = props;
  const isOpen = isDownloadOpen || isUploadOpen;
  if (!isOpen) {
    return null;
  }

  const ctx = React.useMemo(() => new Scp({ clusterId, serverId, login }), [
    clusterId,
    serverId,
    login,
  ]);

  return (
    <ScpContextProvider value={ctx}>
      <FileTransfer
        isDownloadOpen={isDownloadOpen}
        isUploadOpen={isUploadOpen}
        onClose={onClose}
      />
    </ScpContextProvider>
  );
}

export function FileTransfer({
  isDownloadOpen = false,
  isUploadOpen = false,
  onClose,
}: FileTransferProps) {
  const scpContext = useScpContext();
  const { files } = scpContext.store.state;

  function onRemove(id: number) {
    scpContext.removeFile(id);
  }

  function onUpdate(json) {
    scpContext.updateFile(json);
  }

  function onDownload(location: string) {
    scpContext.addDownload(location);
  }

  function onUpload(location: string, filename: string, blob: any) {
    scpContext.addUpload(location, filename, blob);
  }

  function onBeforeClose() {
    const isTransfering = scpContext.isTransfering();
    if (!isTransfering) {
      onClose();
    }

    if (
      isTransfering &&
      window.confirm('Are you sure you want to cancel file transfers?')
    ) {
      onClose();
    }
  }

  function onKeyDown(e) {
    if (e.key !== 'Escape') {
      return;
    }

    e.preventDefault();
    e.stopPropagation();
    onBeforeClose();
  }

  return (
    <StyledFileTransfer onKeyDown={onKeyDown}>
      {isDownloadOpen && <DownloadForm onDownload={onDownload} />}
      {isUploadOpen && <UploadForm onUpload={onUpload} />}
      <FileList onRemove={onRemove} onUpdate={onUpdate} files={files} />
      <ButtonClose onClick={onBeforeClose} />
    </StyledFileTransfer>
  );
}

type FileTransferDialogsProps = {
  clusterId: string;
  isDownloadOpen: boolean;
  isUploadOpen: boolean;
  onClose: () => void;
  login: string;
  serverId: string;
};

type FileTransferProps = {
  isDownloadOpen?: boolean;
  isUploadOpen?: boolean;
  onClose: () => void;
};

const StyledFileTransfer = styled.div`
  background: ${colors.dark};
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.24);
  box-sizing: border-box;
  border: 1px dashed #263238;
  font-size: ${props => props.theme.fontSizes[0]}px;
  color: #28fe14;

  padding: 16px;
  // replace it with the Portal component
  position: absolute;
  right: 0;
  top: 0;
  width: 496px;
  z-index: 2;
`;
