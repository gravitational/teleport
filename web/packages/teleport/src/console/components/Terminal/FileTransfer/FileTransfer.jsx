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

import React, { Component } from 'react';
import styled from 'styled-components';
import PropTypes from 'prop-types';
import DownloadForm from './DownloadForm';
import UploadForm from './UploadForm';
import FileList from './FileList';
import { colors } from './../../colors';
import { CloseButton as TermCloseButton } from './../Elements';

export default class FileTransferDialog extends Component {
  static propTypes = {
    store: PropTypes.object.isRequired,
  };

  componentWillUnmount() {
    this.props.store.close();
  }

  onDownload = location => {
    this.transfer(location, location, false);
  };

  onUpload = (location, filename, blob) => {
    this.transfer(location, filename, true, blob);
  };

  onTransferRemove = id => {
    this.props.store.removeFile(id);
  };

  onTransferUpdate = json => {
    this.props.store.update(json);
  };

  onKeyDown = e => {
    // escape
    if (e.keyCode !== 27) {
      return;
    }

    e.preventDefault();
    e.stopPropagation();

    this.onClose();
  };

  onClose = () => {
    const { store } = this.props;
    const isTransfering = store.isTransfering();
    if (!isTransfering) {
      store.close();
    }

    if (
      isTransfering &&
      window.confirm('Are you sure you want to cancel file transfers?')
    ) {
      store.close();
    }
  };

  transfer(location, name, isUpload, blob = []) {
    this.props.store.addFile({
      location,
      name,
      isUpload,
      blob,
    });
  }

  render() {
    const { state } = this.props.store;
    if (!state.isOpen) {
      return null;
    }

    const { files, isUpload } = state;

    return (
      <StyledFileTransfer onKeyDown={this.onKeyDown}>
        {!isUpload && <DownloadForm onDownload={this.onDownload} />}
        {isUpload && <UploadForm onUpload={this.onUpload} />}
        <FileList
          onRemove={this.onTransferRemove}
          onUpdate={this.onTransferUpdate}
          files={files}
        />
        <CloseButton onClick={this.onClose} />
      </StyledFileTransfer>
    );
  }
}

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

const CloseButton = styled(TermCloseButton)`
  background: #0000;
  color: #fff;
  font-size: ${props => props.theme.fontSizes[4]}px;
  height: 20px;
  opacity: 0.56;
  position: absolute;
  right: 8px;
  top: 8px;
  transition: all 0.3s;
  width: 20px;
  &:hover {
    opacity: 1;
  }
`;
