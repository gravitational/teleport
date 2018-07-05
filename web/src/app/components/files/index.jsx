/*
Copyright 2015 Gravitational, Inc.

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
import connect from './../connect';
import { closeFileTransfer } from 'app/flux/fileTransfer/actions';
import { getters } from 'app/flux/fileTransfer';
import cfg from 'app/config';
import _ from 'lodash';
import { FileToReceive, FileToSend } from './file';
import { DownloadFileInput, UploadFileInput } from './inputPanes';
import { Text } from './items';

class FileTransferContainer extends Component {
  render() {
    const { store, onClose } = this.props;
    if (!store.isOpen) {
      return null;
    }

    return (
      <FileTransfer store={store} onClose={onClose}/>
    )
  }
}

class FileTransfer extends Component {

  constructor() {
    super()
    this.state = {
      files: []
    }
  }

  createFile(name, isUpload, blob) {
    const {
      siteId,
      nodeId,
      login
    } = this.props.store

    let url = cfg.api.getScpUrl({
      siteId,
      nodeId,
      login
    });

    if (name.startsWith('/')) {
      url = `${url}/absolute${name}`
    } else {
      url = `${url}/relative/${name}`
    }

    const id = new Date().getTime() + name;
    return {
      url,
      name,
      isUpload,
      id,
      blob: blob
    }
  }

  onDownloadFile = fileName => {
    let newFile = this.createFile(fileName, false, [])
    this.state.files.unshift(newFile);
    this.setState({});
  }

  onUploadFiles = (remoteLoc, blobs) => {
    if (remoteLoc && remoteLoc[remoteLoc.length - 1] !== '/') {
      remoteLoc = remoteLoc + '/';
    }

    for (var i = 0; i < blobs.length; i++) {
      const name = remoteLoc + blobs[i].name;
      const newFile = this.createFile(name, true, blobs[i]);
      this.state.files.unshift(newFile);
    }

    this.setState({})
  }

  onRemoveFile = id => {
    _.remove(this.state.files, {
      id: id
    })

    this.setState({})
  }

  render() {
    const { onClose, store } = this.props;
    const { files } = this.state;
    const { isUpload } = store;
    return (
      <div className="grv-file-transfer p-sm">
        {!isUpload && <DownloadFileInput onClick={this.onDownloadFile} />}
        {isUpload && <UploadFileInput onSelect={this.onUploadFiles} /> }
        <FileList files={files} onRemove={this.onRemoveFile}/>
        <div className="grv-file-transfer-footer">
          <button onClick={onClose}
            className="btn btn-sm  btn-file-transfer">
            Close
          </button>
        </div>
      </div>
    );
  }
}

const FileList = ({ files, onRemove }) => {
  if (files.length === 0) {
    return null;
  }

  const $files = files.map(file => {
    const props = {
      ...file,
      onRemove: onRemove,
      key: file.id,
    }

    return file.isUpload ?
      <FileToSend {...props} /> :
      <FileToReceive {...props} />
  });

  return (
    <div className="m-t-sm">
      <div className="grv-file-transfer-header m-b-sm">
      </div>
      <div className="grv-file-transfer-file-list-cols">
        <Text> File </Text>
        <Text>Status </Text>
        <div> </div>
      </div>
      <div className="grv-file-transfer-content">
        <div className="grv-file-transfer-file-list">
          {$files}
        </div>
      </div>
    </div>
  )
}

function mapStateToProps() {
  return {
    store: getters.store,
  }
}

function mapActionsToProps() {
  return {
    onClose: closeFileTransfer
  }
}

export default connect(mapStateToProps, mapActionsToProps)(FileTransferContainer);
