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
import styled from 'styled-components';
import PropTypes from 'prop-types';

import { ownerDocument } from './../utils';
import Portal from './Portal';
import RootRef from './RootRef';

export default class Modal extends React.Component {
  mounted = false;

  componentDidMount() {
    this.mounted = true;
    if (this.props.open) {
      this.handleOpen();
    }
  }

  componentDidUpdate(prevProps) {
    if (prevProps.open && !this.props.open) {
      this.handleClose();
    } else if (!prevProps.open && this.props.open) {
      this.lastFocus = ownerDocument(this.mountNode).activeElement;
      this.handleOpen();
    }
  }

  componentWillUnmount() {
    this.mounted = false;
    if (this.props.open) {
      this.handleClose();
    }
  }

  handleOpen = () => {
    const doc = ownerDocument(this.mountNode);
    doc.addEventListener('keydown', this.handleDocumentKeyDown);
    doc.addEventListener('focus', this.enforceFocus, true);

    if (this.dialogRef) {
      this.handleOpened();
    }
  };

  handleOpened = () => {
    this.autoFocus();
    // Fix a bug on Chrome where the scroll isn't initially 0.
    this.modalRef.scrollTop = 0;
  };

  handleClose = () => {
    const doc = ownerDocument(this.mountNode);
    doc.removeEventListener('keydown', this.handleDocumentKeyDown);
    doc.removeEventListener('focus', this.enforceFocus, true);

    this.restoreLastFocus();
  };

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

  handleRendered = () => {
    if (this.props.onRendered) {
      this.props.onRendered();
    }
  };

  handleDocumentKeyDown = event => {
    const ESC = 'Escape';

    // Ignore events that have been `event.preventDefault()` marked.
    if (event.key !== ESC || event.defaultPrevented) {
      return;
    }

    if (this.props.onEscapeKeyDown) {
      this.props.onEscapeKeyDown(event);
    }

    if (!this.props.disableEscapeKeyDown && this.props.onClose) {
      this.props.onClose(event, 'escapeKeyDown');
    }
  };

  enforceFocus = () => {
    // The Modal might not already be mounted.
    if (this.props.disableEnforceFocus || !this.mounted || !this.dialogRef) {
      return;
    }

    const currentActiveElement = ownerDocument(this.mountNode).activeElement;

    if (!this.dialogRef.contains(currentActiveElement)) {
      this.dialogRef.focus();
    }
  };

  handlePortalRef = ref => {
    this.mountNode = ref ? ref.getMountNode() : ref;
  };

  handleModalRef = ref => {
    this.modalRef = ref;
  };

  onRootRef = ref => {
    this.dialogRef = ref;
  };

  autoFocus() {
    // We might render an empty child.
    if (this.props.disableAutoFocus || !this.dialogRef) {
      return;
    }

    const currentActiveElement = ownerDocument(this.mountNode).activeElement;

    if (!this.dialogRef.contains(currentActiveElement)) {
      if (!this.dialogRef.hasAttribute('tabIndex')) {
        this.dialogRef.setAttribute('tabIndex', -1);
      }

      this.lastFocus = currentActiveElement;
      this.dialogRef.focus();
    }
  }

  restoreLastFocus() {
    if (this.props.disableRestoreFocus || !this.lastFocus) {
      return;
    }

    // Not all elements in IE 11 have a focus method.
    // Because IE 11 market share is low, we accept the restore focus being broken
    // and we silent the issue.
    if (this.lastFocus.focus) {
      this.lastFocus.focus();
    }

    this.lastFocus = null;
  }

  render() {
    const {
      BackdropProps,
      children,
      container,
      disablePortal,
      modalCss,
      hideBackdrop,
      open,
      className,
    } = this.props;

    const childProps = {};

    if (!open) {
      return null;
    }

    return (
      <Portal
        ref={this.handlePortalRef}
        container={container}
        disablePortal={disablePortal}
        onRendered={this.handleRendered}
        data-testid="portal"
      >
        <StyledModal
          modalCss={modalCss}
          data-testid="Modal"
          ref={this.handleModalRef}
          className={className}
        >
          {!hideBackdrop && (
            <Backdrop onClick={this.handleBackdropClick} {...BackdropProps} />
          )}
          <RootRef rootRef={this.onRootRef}>
            {React.cloneElement(children, childProps)}
          </RootRef>
        </StyledModal>
      </Portal>
    );
  }
}

Modal.propTypes = {
  /**
   * Properties applied to the [`Backdrop`](/api/backdrop/) element.
   *
   * invisible: Boolean - allows backdrop to keep bg color of parent eg: popup menu
   */
  BackdropProps: PropTypes.object,
  /**
   * A single child content element.
   */
  children: PropTypes.element,
  /**
   * A node, component instance, or function that returns either.
   * The `container` will have the portal children appended to it.
   */
  container: PropTypes.oneOfType([PropTypes.object, PropTypes.func]),
  /**
   * If `true`, the modal will not automatically shift focus to itself when it opens, and
   * replace it to the last focused element when it closes.
   * This also works correctly with any modal children that have the `disableAutoFocus` prop.
   *
   * Generally this should never be set to `true` as it makes the modal less
   * accessible to assistive technologies, like screen readers.
   */
  disableAutoFocus: PropTypes.bool,
  /**
   * If `true`, clicking the backdrop will not fire any callback.
   */
  disableBackdropClick: PropTypes.bool,
  /**
   * If `true`, the modal will not prevent focus from leaving the modal while open.
   *
   * Generally this should never be set to `true` as it makes the modal less
   * accessible to assistive technologies, like screen readers.
   */
  disableEnforceFocus: PropTypes.bool,
  /**
   * If `true`, hitting escape will not fire any callback.
   */
  disableEscapeKeyDown: PropTypes.bool,
  /**
   * Disable the portal behavior.
   * The children stay within it's parent DOM hierarchy.
   */
  disablePortal: PropTypes.bool,
  /**
   * If `true`, the modal will not restore focus to previously focused element once
   * modal is hidden.
   */
  disableRestoreFocus: PropTypes.bool,
  /**
   * If `true`, the backdrop is not rendered.
   */
  hideBackdrop: PropTypes.bool,
  /**
   * Callback fired when the backdrop is clicked.
   */
  onBackdropClick: PropTypes.func,
  /**
   * Callback fired when the component requests to be closed.
   * The `reason` parameter can optionally be used to control the response to `onClose`.
   *
   * @param {object} event The event source of the callback
   * @param {string} reason Can be:`"escapeKeyDown"`, `"backdropClick"`
   */
  onClose: PropTypes.func,
  /**
   * Callback fired when the escape key is pressed,
   * `disableEscapeKeyDown` is false and the modal is in focus.
   */
  onEscapeKeyDown: PropTypes.func,
  /**
   * Callback fired once the children has been mounted into the `container`.
   * It signals that the `open={true}` property took effect.
   */
  onRendered: PropTypes.func,
  /**
   * If `true`, the modal is open.
   */
  open: PropTypes.bool.isRequired,
  className: PropTypes.string,
};

Modal.defaultProps = {
  disableAutoFocus: false,
  disableBackdropClick: false,
  disableEnforceFocus: false,
  disableEscapeKeyDown: false,
  disablePortal: false,
  disableRestoreFocus: false,
  hideBackdrop: false,
};

function Backdrop(props) {
  const { invisible, ...rest } = props;
  return (
    <StyledBackdrop
      data-testid="backdrop"
      aria-hidden="true"
      invisible={invisible}
      {...rest}
    />
  );
}

const StyledBackdrop = styled.div`
  z-index: -1;
  position: fixed;
  right: 0;
  bottom: 0;
  top: 0;
  left: 0;
  background-color: ${props =>
    props.invisible ? 'transparent' : `rgba(0, 0, 0, 0.5)`};
  opacity: 1;
  touch-action: none;
`;

const StyledModal = styled.div`
  position: fixed;
  z-index: 1200;
  right: 0;
  bottom: 0;
  top: 0;
  left: 0;
  ${props => props.modalCss && props.modalCss(props)}
`;
