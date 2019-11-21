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
import PropTypes from 'prop-types';
import { colors } from 'teleport/console/components/colors';
import DownloadForm from './DownloadForm';
import UploadForm from './UploadForm';
import FileList from './FileList';
import { ButtonClose } from './Elements';

export default function FileTransferDialog(props) {
  const {
    isDownloadOpen,
    isUploadOpen,
    files,
    onDownload,
    onUpload,
    onRemove,
    onUpdate,
    onClose,
  } = props;
  const isOpen = isDownloadOpen || isUploadOpen;

  if (!isOpen) {
    return null;
  }

  function onKeyDown(e) {
    // escape
    if (e.keyCode !== 27) {
      return;
    }

    e.preventDefault();
    e.stopPropagation();
    onClose();
  }

  return (
    <StyledFileTransfer onKeyDown={onKeyDown}>
      {isDownloadOpen && <DownloadForm onDownload={onDownload} />}
      {isUploadOpen && <UploadForm onUpload={onUpload} />}
      <FileList onRemove={onRemove} onUpdate={onUpdate} files={files} />
      <ButtonClose onClick={onClose} />
    </StyledFileTransfer>
  );
}

FileTransferDialog.propTypes = {
  isDownloadOpen: PropTypes.bool.isRequired,
  isUploadOpen: PropTypes.bool.isRequired,
  files: PropTypes.array.isRequired,
  onDownload: PropTypes.func.isRequired,
  onUpload: PropTypes.func.isRequired,
  onRemove: PropTypes.func.isRequired,
  onUpdate: PropTypes.func.isRequired,
  onClose: PropTypes.func.isRequired,
};

const StyledFileTransfer = styled.div`
  background: ${colors.dark};
  border-radius: 4px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.24);
  box-sizing: border-box;
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
