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
import PropTypes from 'prop-types';
import * as Elements from './../Elements';
import { Flex } from 'design';

export default class FileDownloadSelector extends React.Component {

  static propTypes = {
    onDownload: PropTypes.func.isRequired,
  }

  state = {
    path: '~/'
  }

  onChangePath = e => {
    this.setState({
      path: e.target.value
    })
  }

  isValidPath(path) {
    return path && path[path.length - 1] !== '/';
  }

  onDownload = () => {
    if (this.isValidPath(this.state.path)) {
      this.props.onDownload(this.state.path)
    }
  }

  onKeyDown = event => {
    if (event.key === 'Enter') {
      event.preventDefault();
      event.stopPropagation();
      this.onDownload();
    }
  }

  moveCaretAtEnd(e) {
    const tmp = e.target.value;
    e.target.value = '';
    e.target.value = tmp;
  }

  render() {
    const { path } = this.state;
    const isBtnDisabled = !this.isValidPath(path);
    return (
      <Elements.Form>
        <Elements.Header>
          (SCP) Download Files
        </Elements.Header>
        <Elements.Label>File Path</Elements.Label>
        <Flex>
          <Elements.Input onChange={this.onChangePath}
            ref={e => this.inputRef = e}
            value={path}
            mb={0}
            autoFocus
            onFocus={this.moveCaretAtEnd}
            onKeyDown={this.onKeyDown}
          />
          <Elements.Button
            disabled={isBtnDisabled}
            onClick={this.onDownload}>
            Download
          </Elements.Button>
        </Flex>
      </Elements.Form>
    )
  }
}