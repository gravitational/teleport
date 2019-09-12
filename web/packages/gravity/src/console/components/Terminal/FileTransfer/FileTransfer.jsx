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
    onTransferRemove: PropTypes.func.isRequired,
    onTransferStart: PropTypes.func.isRequired,
    onTransferUpdate: PropTypes.func.isRequired,
    onClose: PropTypes.func.isRequired
  }

  transfer(location, name, isUpload, blob=[]) {
    this.props.onTransferStart({
      location,
      name,
      isUpload,
      blob
    })
  }

  componentWillUnmount(){
    this.props.onClose();
  }

  onDownload = location => {
    this.transfer(location, location, false)
  }

  onUpload = (location, filename, blob) => {
    this.transfer(location, filename, true, blob);
  }

  onKeyDown = e => {
    // escape
    if (e.keyCode !== 27) {
      return;
    }

    e.preventDefault();
    e.stopPropagation();

    this.onClose();
  }

  onClose = () => {
    const isTransfering = this.props.store.isTransfering();
    if (!isTransfering) {
      this.props.onClose();
    }

    if (isTransfering && window.confirm("Are you sure you want to cancel file transfers?")) {
      this.props.onClose();
    }
  }

  render() {
    const { store, onTransferUpdate, onTransferRemove } = this.props;
    if (!store.isOpen) {
      return null;
    }

    const { files, isUpload } = store;
    const latestFirst = files.toArray().reverse();
    return (
      <StyledFileTransfer onKeyDown={this.onKeyDown}>
        {!isUpload && <DownloadForm onDownload={this.onDownload} />}
        {isUpload && <UploadForm onUpload={this.onUpload} /> }
        <FileList
          onRemove={onTransferRemove}
          onUpdate={onTransferUpdate}
          files={latestFirst} />
        <CloseButton onClick={this.onClose} />
      </StyledFileTransfer>
    )
  }
}

const StyledFileTransfer = styled.div`
  background: ${colors.dark};
  border-radius: 4px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, .24);
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
`

const CloseButton = styled(TermCloseButton)`
  background: #0000;
  color: #fff;
  font-size: ${props => props.theme.fontSizes[4]}px;
  height: 20px;
  opacity: .56;
  position: absolute;
  right: 8px;
  top: 8px;
  transition: all .3s;
  width: 20px;
  &:hover {
    opacity: 1;
  }
`