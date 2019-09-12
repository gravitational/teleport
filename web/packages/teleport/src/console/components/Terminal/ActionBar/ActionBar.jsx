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
import PropTypes from 'prop-types'
import styled from 'styled-components';
import * as Icons from 'design/Icon';
import { Flex, Text } from 'design';
import { CloseButton } from './../Elements';

export default class ActionBar extends React.Component {

  close = () => {
    this.props.onClose && this.props.onClose();
  }

  openUploadDialog = () => {
    this.openFileTransferDialog(true);
  }

  openDownloadDialog = () => {
    this.openFileTransferDialog(false);
  }

  render() {
    const {
      isFileTransferDialogOpen,
      onOpenUploadDialog,
      onOpenDownloadDialog,
      title
    } = this.props;

    return (
      <Flex height="32px" my={1} alignItems="flex-start">
        <Tab>
          <CloseButton mr={2} onClick={this.close}/>
          <Text maxWidth={2} fontSize={1}>{title}</Text>
        </Tab>
        <IconButton
          title="Download files"
          disabled={isFileTransferDialogOpen}
          onClick={onOpenDownloadDialog}>
          <Icons.Download />
        </IconButton>
        <IconButton
          title="Upload files"
          disabled={isFileTransferDialogOpen}
          onClick={onOpenUploadDialog}>
          <Icons.Upload />
        </IconButton>
      </Flex>
    )
  }
}

ActionBar.propTypes = {
  isFileTransferDialogOpen: PropTypes.bool.isRequired,
  onClose: PropTypes.func.isRequired,
  onOpenDownloadDialog: PropTypes.func.isRequired,
  onOpenUploadDialog: PropTypes.func.isRequired,
  title: PropTypes.string.isRequired,
};

const isOpen = props => {
  if (props.disabled) {
    return {
      opacity: 0.24,
      cursor: "not-allowed"
    }
  }
}

const Tab = ({ children }) => (
  <Flex mr={3} py={1} alignItems="center" children={children} />
)

const IconButton = styled.button`
  background: none;
  border: none;
  border-radius: 2px;
  width: 24px;
  height: 32px;
  color: rgba(255, 255, 255, 0.56);
  cursor: pointer;
  font-size:  ${props => props.theme.fontSizes[4]}px;
  display: flex;
  opacity: .87;
  outline: none;
  align-items: center;
  justify-content: center;
  ${isOpen};
`