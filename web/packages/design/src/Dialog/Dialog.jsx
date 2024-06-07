/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React from 'react';
import PropTypes from 'prop-types';
import styled from 'styled-components';

import Modal from './../Modal';

export default class Dialog extends React.Component {
  render() {
    const { children, dialogCss, className, ...modalProps } = this.props;
    return (
      <Modal role="dialog" {...modalProps}>
        <ModalBox>
          <DialogBox
            data-testid="dialogbox"
            dialogCss={dialogCss}
            className={className}
          >
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
  className: PropTypes.string,
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
