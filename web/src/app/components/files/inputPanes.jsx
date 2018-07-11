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

import React from 'react';
import { Text } from './items';

export class DownloadFileInput extends React.Component {

  state = {
    path: ''
  }

  onChangePath = e => {
    this.setState({
      path: e.target.value
    })
  }

  onClick = () => {
    if (this.state.path) {
      this.props.onClick(this.state.path)
      this.inputRef.focus();
    }
  }

  onKeyDown = event => {
    if (event.key === 'Enter') {
      event.preventDefault();
      event.stopPropagation();
      this.onClick();
    }
  }

  render() {
    const isBtnDisabled = !this.state.path;
    return (
      <div className="grv-file-transfer-header">
        <h4 className="grv-file-transfer-title">DOWNLOAD A FILE</h4>
        <Text className="m-b-xs">
          Full path of file
        </Text>
        <div className="grv-file-transfer-download">
          <input onChange={this.onChangePath}
            ref={e => this.inputRef = e}
            className="grv-file-transfer-input m-r-sm"
            placeholder="Fully qualified file path"
            autoFocus
            onKeyDown={this.onKeyDown}
          />
          <button className="btn btn-sm grv-file-transfer-btn"
            style={{width:"105px"}}
            disabled={isBtnDisabled}
            onClick={this.onClick}>
            Download
          </button>
        </div>
      </div>
    )
  }
}

export class UploadFileInput extends React.Component {

  state = {
    files: [],
    remoteLocation: "./"
  }

  onSelected = e => {
    const blobs = e.target.files;
    const files = [];
    for (var i = 0; i < blobs.length; i++) {
        files.push(blobs[i]);
    }

    //const remoteLocation = this.inputRef.value;
    this.props.onSelect(this.state.remoteLocation, e.target.files);
    // reset all selected files
    e.target.value = "";
    this.inputRef.focus();
  }

  onLocationChanged = e => {
    //const remoteLocation = this.inputRef.value;
    //this.props.onSelect(remoteLocation, e.target.files);
    // reset all selected files
    this.setState({
      remoteLocation: e.target.value
    })
  }

  onUploadClick = () => {
    const { files, remoteLocation } = this.state;
    this.props.onSelect(remoteLocation, files);
    // reset all selected files
  }

  onClick = () => {
    //this.fileSelectorRef.value = ""
    this.fileSelectorRef.click();
    // set a focus back on input field
    //this.inputRef.focus();
  }

  onKeyDown = event => {
    if (event.key === 'Enter') {
      event.preventDefault();
      event.stopPropagation();
      this.onClick();
    }
  }

  moveCaretAtEnd(e) {
    const tmp = e.target.value;
    e.target.value = '';
    e.target.value = tmp;
  }

  drop = e => {
    e.preventDefault();
    e.stopPropagation();
    if (e.dataTransfer.files.length === 0) {
      return;
    }

    this.props.onSelect(this.state.remoteLocation, e.dataTransfer.files);
  }

  render() {
    const { remoteLocation, files } = this.state;
    //const isDldBtnDisabled = !remoteLocation || files.length === 0;
    const hasFiles = files.length > 0;

    return (
      <div className="grv-file-transfer-header">
        <h4 className="grv-file-transfer-title">UPLOAD FILES</h4>
        <Text className="m-b-xs">
          Enter the location to upload files
        </Text>
        <div className="grv-file-transfer-upload">
          <input className="grv-file-transfer-input m-r-sm"
            ref={e => this.inputRef = e}
            value={remoteLocation}
            placeholder=""
            autoFocus
            onFocus={this.moveCaretAtEnd}
            onChange={this.onLocationChanged}
            onKeyDown={this.onKeyDown}
          />

          <div className="grv-file-transfer-upload-selected-files m-t"
            onDragOver={e => e.preventDefault()}
            onDrop={this.drop}
          >
            {!hasFiles &&
              <div>
                <a onClick={this.onClick}>Select files</a> or place them here
              </div>
            }
            {hasFiles &&
              <div>
                <a onClick={this.onClick}> {files.length} files selected </a>
              </div>
            }
          </div>
          <input ref={e => this.fileSelectorRef = e} type="file"
            multiple
            style={{ display: "none" }}
            accept="*.*"
            name="file"
            onChange={this.onSelected}
          />
        </div>
        {/*<div className="grv-file-transfer-footer m-t">
           <button
            onClick={this.onUploadClick}
            disabled={isDldBtnDisabled}
            className="btn btn-sm  grv-file-transfer-btn m-r">
            Upload
         </button>
          <button
            className="btn btn-sm  grv-file-transfer-btn">
            Close
          </button>
        </div>*/}
      </div>
    )
  }
}
