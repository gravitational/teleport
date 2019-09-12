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
import { ownerDocument } from './../utils';
import Portal from './Portal';
import RootRef from './RootRef';

function getHasTransition(props) {
  return props.children ? props.children.props.hasOwnProperty('in') : false;
}

class Modal extends React.Component {

  mounted = false;

  constructor(props) {
    super();
    this.state = {
      exited: !props.open,
    };
  }

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

    if (this.props.open || (getHasTransition(this.props) && !this.state.exited)) {
      this.handleClose();
    }
  }

  static getDerivedStateFromProps(nextProps) {
    if (nextProps.open) {
      return {
        exited: false,
      };
    }

    return null;
  }

  handleOpen = () => {
    const doc = ownerDocument(this.mountNode);
    //const container = getContainer(this.props.container, doc.body);

    //this.props.manager.add(this, container);
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

  handleExited = () => {
    this.setState({ exited: true });
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

  handleDocumentKeyDown = event => {
    const ESC = 27;

    // Ignore events that have been `event.preventDefault()` marked.
    if (event.which !== ESC || !this.isTopModal() || event.defaultPrevented) {
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
    if (!this.isTopModal() || this.props.disableEnforceFocus || !this.mounted || !this.dialogRef) {
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

  isTopModal() {
    return true;
  }

  render() {
    const {
      BackdropProps,
      children,
      container,
      disablePortal,
      modalCss,
      hideBackdrop,
      open
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
      >
        <StyledModal modalCss={modalCss} data-mui-test="Modal" ref={this.handleModalRef}>
          { !hideBackdrop && <Backdrop onClick={this.handleBackdropClick} {...BackdropProps} /> }
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
   * A backdrop component. This property enables custom backdrop rendering.
   */

  /**
   * Properties applied to the [`Backdrop`](/api/backdrop/) element.
   */
  BackdropProps: PropTypes.object,
  /**
   * A single child content element.
   */
  children: PropTypes.element,

  /**
   * @ignore
   */
  className: PropTypes.string,
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
   * Always keep the children in the DOM.
   * This property can be useful in SEO situation or
   * when you want to maximize the responsiveness of the Modal.
   */
  keepMounted: PropTypes.bool,
  /**
   * A modal manager used to track and manage the state of open
   * Modals. This enables customizing how modals interact within a container.
   */
  manager: PropTypes.object,
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
};

Modal.defaultProps = {
  BackdropComponent: Backdrop,
  disableAutoFocus: false,
  disableBackdropClick: false,
  disableEnforceFocus: false,
  disableEscapeKeyDown: false,
  disablePortal: false,
  disableRestoreFocus: false,
  hideBackdrop: false,
  keepMounted: false,
};

export default Modal;

const Backdrop = props => {
  const { invisible, ...rest } = props;
  return (
    <StyledBackdrop
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
  background-color: ${ props => props.invisible ? 'transparent' : `rgba(0, 0, 0, 0.5)`};
  opacity: 1;
  touch-action: none;
`

const StyledModal = styled.div`
  position: fixed;
  z-index: 1200;
  right: 0;
  bottom: 0;
  top: 0;
  left: 0;
  ${ props => props.modalCss && props.modalCss(props) }
`