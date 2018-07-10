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
import classnames from 'classnames';
import { Uploader, Downloader } from 'app/services/fileTransfer';
import { Text } from './items';
import withHttpRequest from './withHttpRequest';

export class File extends Component {

  savedToDisk = false;

  onRemove = () => {
    this.props.onRemove(this.props.id);
  }

  componentDidUpdate() {
    const { isCompleted, response } = this.props.status;
    if (isCompleted && !this.props.isUpload) {
      this.saveToDisk(response)
    }
  }

  saveToDisk({ fileName, blob }) {
    if (this.savedToDisk) {
      return;
    }

    this.savedToDisk = true;

    const a = document.createElement("a");
    a.href = window.URL.createObjectURL(blob);
    a.download = fileName;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  }

  render() {
    let {
      isFailed,
      isProcessing,
      isCompleted,
      progress,
      error
    } = this.props.status;

    const { name } = this.props;
    const className = classnames(
      "grv-file-transfer-file-list-item",
      isFailed && "--failed",
      isProcessing && "--processing",
      isCompleted && "--completed",
    )

    return (
      <div className={className}>
        <div className="grv-file-transfer-file-path">
          {name}
          {isFailed && <div> {error} </div> }
        </div>
        <div className="grv-file-transfer-file-status">
          {isFailed &&
            <div>
              failed
            </div>
          }
          {isProcessing &&
            <div>
              {progress}%
            </div>
          }
          {isCompleted &&
            <Text>completed</Text>
          }
        </div>
        <div className="grv-file-transfer-file-close">
          <a onClick={this.onRemove}>
            <i className="fa fa-times" aria-hidden="true"></i>
          </a>
        </div>
      </div>
    )
  }
}

export const FileToSend = withHttpRequest(Uploader)(File);
export const FileToReceive = withHttpRequest(Downloader)(File);