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
import * as Elements from './../Elements';
import { colors } from '../../../colors';

export default class UploadForm extends React.Component {

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
    const dropZoneText = hasFiles ? `${files.length} files selected` :
      `Select files to upload or drag & drop them here`;

    return (
      <Elements.Form color="terminal">
        <Elements.Header>(SCP) UPLOAD Files</Elements.Header>
        <Elements.Label>Upload destination </Elements.Label>
        <Elements.Input className="grv-file-transfer-input m-r-sm"
          width="100%"
          mb={0}
          ref={e => this.inputRef = e}
          value={remoteLocation}
          autoFocus
          onFocus={this.moveCaretAtEnd}
          onChange={this.onFilePathChanged}
          onKeyDown={this.onKeyDown}
        />
        <input ref={e => this.fileSelectorRef = e} type="file"
          multiple
          style={{ display: "none" }}
          accept="*.*"
          name="file"
          onChange={this.onFileSelected}
        />
        <Dropzone
          ref={ e => this.refDropzone = e }
          onDragOver={e => e.preventDefault()}
          onDrop={this.onDrop}>
          <a onClick={this.onOpenFilePicker}>
            {dropZoneText}
          </a>
        </Dropzone>
        <Elements.Button disabled={isDldBtnDisabled} onClick={this.onUpload}>
          Upload
        </Elements.Button>
      </Elements.Form>
    )
  }
}

const Dropzone = styled.div`
  background: ${colors.bgTerminal};
  border: 1px dashed ${colors.text};
  color: ${colors.terminal};
  display: block;
  margin: 16px 0;
  height: 72px;
  line-height: 72px;
  text-align: center;
  text-transform: uppercase;
`