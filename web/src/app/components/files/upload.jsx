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

import React, { PropTypes } from 'react';
import { Text } from './items';

export class FileUploadSelector extends React.Component {

  static propTypes = {
    onUpload: PropTypes.func.isRequired,
  }

  state = {
    files: [],
    remoteLocation: "~/"
  }

  componentWillUnmount() {
    document.removeEventListener('drop', this.onDocumentDrop);
    document.removeEventListener('dragover', this.preventDefault);
  }

  componentDidMount() {
    document.addEventListener('dragover', this.preventDefault, false);
    document.addEventListener('drop', this.onDocumentDrop, false);
  }

  preventDefault(e) {
    e.preventDefault();
  }

  onDocumentDrop(e) {
    if (this.refDropzone && this.refDropzone.contains(e.target)) {
      return
    }

    e.preventDefault();
    e.dataTransfer.effectAllowed = 'none';
    e.dataTransfer.dropEffect = 'none';
  }

  onFileSelected = e => {
    this.addFiles([], e.target.files);
    this.inputRef.focus();
  }

  onFilePathChanged = e => {
    this.setState({
      remoteLocation: e.target.value
    })
  }

  onUpload = () => {
    const { files, remoteLocation } = this.state;
    for (var i = 0; i < files.length; i++) {
      this.props.onUpload(
        remoteLocation,
        files[i].name,
        files[i]);
    }

    this.setState({ files: [] });
    this.setFocus();
  }

  onOpenFilePicker = () => {
    // reset all selected files
    this.fileSelectorRef.value = "";
    this.fileSelectorRef.click();
  }

  onDrop = e => {
    e.preventDefault();
    e.stopPropagation();
    this.addFiles(this.state.files, e.dataTransfer.files)
    this.setFocus();
  }

  onKeyDown = event => {
    if (event.key === 'Enter') {
      event.preventDefault();
      event.stopPropagation();
      this.onOpenFilePicker();
    }
  }

  setFocus() {
    this.inputRef.focus();
  }

  moveCaretAtEnd(e) {
    const tmp = e.target.value;
    e.target.value = '';
    e.target.value = tmp;
  }

  addFiles(files, blobs = []) {
    for (var i = 0; i < blobs.length; i++) {
      files.push(blobs[i]);
    }

    this.setState({
      files
    })
  }

  render() {
    const { remoteLocation, files } = this.state;
    const isDldBtnDisabled = !remoteLocation || files.length === 0;
    const hasFiles = files.length > 0;

    return (
      <div className="grv-file-transfer-header m-b">
        <Text className="m-b">
          <h4>SCP UPLOAD</h4>
        </Text>
        <div className="grv-file-transfer-upload">
          <div className="grv-file-transfer-upload-selected-files"
            ref={ e => this.refDropzone = e }
            onDragOver={e => e.preventDefault()}
            onDrop={this.onDrop}
          >
            {!hasFiles &&
              <div>
                <a onClick={this.onOpenFilePicker}>Select files</a> to upload or drag & drop them here
              </div>
            }
            {hasFiles &&
              <div>
                <a onClick={this.onOpenFilePicker}> {files.length} files selected </a>
              </div>
            }
          </div>
          <Text className="m-b-xs m-t" >
            Upload destination
          </Text>
          <div style={{ display: "flex" }}>
            <input className="grv-file-transfer-input m-r-sm"
              ref={e => this.inputRef = e}
              value={remoteLocation}
              placeholder=""
              autoFocus
              onFocus={this.moveCaretAtEnd}
              onChange={this.onFilePathChanged}
              onKeyDown={this.onKeyDown}
            />
            <button className="btn btn-sm grv-file-transfer-btn"
              style={{ width: "105px" }}
              disabled={isDldBtnDisabled}
              onClick={this.onUpload}>
              Upload
            </button>
          </div>
          <input ref={e => this.fileSelectorRef = e} type="file"
            multiple
            style={{ display: "none" }}
            accept="*.*"
            name="file"
            onChange={this.onFileSelected}
          />
        </div>
      </div>
    )
  }
}