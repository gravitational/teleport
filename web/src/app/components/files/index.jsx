/*
Copyright 2018 Gravitational, Inc.

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

import React, { Component, PropTypes } from 'react';
import { FileDownloadSelector } from './download';
import { FileUploadSelector } from './upload';
import { FileTransfer } from './fileTransfer';

export class FileTransferDialog extends Component {

  static propTypes = {
    store: PropTypes.object.isRequired,
    onTransfer: PropTypes.func.isRequired,
    onClose: PropTypes.func.isRequired
  }

  transfer(location, name, isUpload, blob=[]) {
    this.props.onTransfer({
      location,
      name,
      isUpload,
      blob
    })
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
    const { store } = this.props;
    if (!store.isOpen) {
      return null;
    }

    const { files, isUpload } = store;
    const latestFirst = files.toArray().reverse();
    return (
      <div className="grv-file-transfer p-sm" onKeyDown={this.onKeyDown}>
        {!isUpload && <FileDownloadSelector onDownload={this.onDownload} />}
        {isUpload && <FileUploadSelector onUpload={this.onUpload} /> }
        <FileTransfer files={latestFirst}/>
        <div className="grv-file-transfer-footer">
          <button onClick={this.onClose}
            className="btn btn-sm  grv-file-transfer-btn">
            Close
          </button>
        </div>
      </div>
    )
  }
}
