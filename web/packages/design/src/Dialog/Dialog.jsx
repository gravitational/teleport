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
import Modal from './../Modal';
import styled from 'styled-components';

class Dialog extends React.Component {

  handleBackdropClick = event => {

    if (event.target !== event.currentTarget) {
      return;
    }

    if (this.props.onBackdropClick) {
      this.props.onBackdropClick(event);
    }

    if (!this.props.disableBackdropClick && this.props.onClose) {
      this.props.onClose(event, 'backdropClick');
    }
  };

  render() {
    const {
      children,
      disableBackdropClick,
      disableEscapeKeyDown,
      onBackdropClick,
      onClose,
      onEscapeKeyDown,
      open,
      dialogCss,
      ...other
    } = this.props;

    return (
      <Modal
        disableBackdropClick={disableBackdropClick}
        disableEscapeKeyDown={disableEscapeKeyDown}
        onBackdropClick={onBackdropClick}
        onEscapeKeyDown={onEscapeKeyDown}
        onClose={onClose}
        open={open}
        role="dialog"
        {...other}
      >
        <ModalBox>
          <DialogBox dialogCss={dialogCss}>
            {children}
          </DialogBox>
        </ModalBox>
      </Modal>
    );
  }
}

Dialog.defaultProps = {
  disableBackdropClick: false,
  disableEscapeKeyDown: false,
  fullScreen: false,
  fullWidth: false,
};

const ModalBox = styled.div`
  height: 100%;
  outline: none;
  color: black;
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 1;
  will-change: opacity;
  transition: opacity 225ms cubic-bezier(0.4, 0, 0.2, 1) 0ms;
`

const DialogBox = styled.div`
  padding: 32px;
  padding-top: 24px;
  background: ${props => props.theme.colors.primary.main };
  color: ${props => props.theme.colors.text.primary};
  border-radius: 8px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, .24);
  display: flex;
  flex-direction: column;
  position: relative;
  overflow-y: auto;
  max-height: calc(100% - 96px);
  ${props => props.dialogCss && props.dialogCss(props) }

`

export default Dialog;