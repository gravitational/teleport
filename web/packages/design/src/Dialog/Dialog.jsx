/*
Copyright 2019-2020 Gravitational, Inc.

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
import styled from 'styled-components';

import Modal from './../Modal';

export default class Dialog extends React.Component {
  render() {
    const { children, dialogCss, ...modalProps } = this.props;
    return (
      <Modal role="dialog" {...modalProps}>
        <ModalBox>
          <DialogBox data-testid="dialogbox" dialogCss={dialogCss}>
            {children}
          </DialogBox>
        </ModalBox>
      </Modal>
    );
  }
}

Dialog.defaultProps = {
  disableBackdropClick: true,
  disableEscapeKeyDown: true,
};

Dialog.propTypes = {
  ...Modal.propTypes,
  children: PropTypes.node,
  dialogCss: PropTypes.func,
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
`;

const DialogBox = styled.div`
  padding: 32px;
  padding-top: 24px;
  background: ${props => props.theme.colors.levels.surface};
  color: ${props => props.theme.colors.text.main};
  border-radius: 8px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.24);
  display: flex;
  flex-direction: column;
  position: relative;
  overflow-y: auto;
  max-height: calc(100% - 96px);
  ${props => props.dialogCss && props.dialogCss(props)};
`;
