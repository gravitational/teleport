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
      <div className="grv-file-transfer-header m-b">
        <Text className="m-b">
          <h4>DOWNLOAD A FILE</h4>
        </Text>
        <Text className="m-b-xs">
          Full path of file
        </Text>
        <div className="grv-file-transfer-download">
          <input onChange={this.onChangePath}
            className="grv-file-transfer-input m-r-sm"
            placeholder="Fully qualified file path"
            autoFocus
            onKeyDown={this.onKeyDown}
          />
          <button className="btn btn-sm btn-file-transfer"
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
  onSelected = e => {

    const remoteLocation = this.inputRef.value;
    this.props.onSelect(remoteLocation, e.target.files);
    // reset all selected files
    e.target.value = "";
  }

  onClick = () => {
    this.fileSelectorRef.click();
  }

  onKeyDown = event => {
    if (event.key === 'Enter') {
      event.preventDefault();
      event.stopPropagation();
      this.onClick();
    }
  }

  render() {
    return (
      <div className="grv-file-transfer-header m-b">
        <Text className="m-b">
          <h4>UPLOAD FILES</h4>
        </Text>
        <Text className="m-b-xs">
          Enter the location to upload files
        </Text>
        <div className="grv-file-transfer-download">
          <input ref={e => { this.inputRef = e }}
            className="grv-file-transfer-input m-r-sm"
            placeholder=""
            autoFocus
            onKeyDown={this.onKeyDown}
          />
          <button className="btn btn-sm btn-file-transfer" onClick={this.onClick}>
            Select files...
          </button>
          <input ref={e => this.fileSelectorRef = e} type="file"
            multiple
            style={{ display: "none" }}
            accept="*.*"
            name="file"
            onChange={this.onSelected}
          />
        </div>
      </div>
    )
  }
}



