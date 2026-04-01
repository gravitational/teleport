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

import React, { cloneElement, ComponentProps, Ref, RefObject } from 'react';
import { createPortal } from 'react-dom';
import styled, { StyleFunction } from 'styled-components';

export type ModalProps = {
  /**
   * If `true`, the modal is open.
   */
  open: boolean;
  /**
   * Prevent unmounting the component and its children from the DOM when closed.
   * Instead, hides it with CSS.
   */
  keepInDOMAfterClose?: boolean;

  className?: string;

  /**
   * Styles passed to the modal, the parent of the children.
   */
  modalCss?: StyleFunction<ComponentProps<'div'>>;

  /**
   * The child must be a single HTML element. Modal used to call methods such as focus and
   * setAttribute on the outermost element. This is no longer the case, so technically this type can
   * be adjusted if needed.
   */
  children?: React.ReactElement;

  /**
   * Properties applied to the Backdrop element.
   */
  BackdropProps?: BackdropProps;

  /**
   * If `true`, clicking the backdrop will not fire any callback.
   */
  disableBackdropClick?: boolean;

  /**
   * If `true`, hitting escape will not fire any callback.
   */
  disableEscapeKeyDown?: boolean;

  /**
   * If `true`, the modal will not restore focus to previously focused element once
   * modal is hidden.
   */
  disableRestoreFocus?: boolean;

  /**
   * If `true`, the backdrop is not rendered.
   */
  hideBackdrop?: boolean;

  /**
   * Callback fired when the backdrop is clicked.
   */
  onBackdropClick?: (event: React.MouseEvent) => void;

  /**
   * Callback fired when the component requests to be closed.
   * The `reason` parameter can optionally be used to control the response to `onClose`.
   */
  onClose?: (
    event: KeyboardEvent | React.MouseEvent,
    reason: 'escapeKeyDown' | 'backdropClick'
  ) => void;

  /**
   * Callback fired when the escape key is pressed,
   * `disableEscapeKeyDown` is false and the modal is in focus.
   */
  onEscapeKeyDown?: (event: KeyboardEvent) => void;

  /**
   * A ref to the modal container, as opposed to `ref`, which captures the
   * dialog itself.
   */
  modalRef?: Ref<HTMLDivElement | null>;

  /**
   * If `true`, focus is trapped inside the modal. When something outside the modal tries to grab
   * focus programmatically, focus is returned to the last focused element inside the modal.
   * Tab/Shift+Tab cycling wraps around at the modal boundaries.
   */
  trapFocus?: boolean;
};

export default class Modal extends React.Component<ModalProps> {
  lastFocus: HTMLElement | undefined;
  lastModalFocus: HTMLElement | undefined;
  modalEl: HTMLDivElement | null = null;
  mounted = false;
  lastTabKeyDirection: 'forward' | 'backward' | null = null;

  componentDidMount() {
    this.mounted = true;
    if (this.props.open) {
      this.handleOpen();
    }
  }

  componentDidUpdate(prevProps: ModalProps) {
    if (prevProps.open && !this.props.open) {
      this.handleClose();
    } else if (!prevProps.open && this.props.open) {
      this.lastFocus = document.activeElement as HTMLElement;
      this.handleOpen();
    }
  }

  componentWillUnmount() {
    this.mounted = false;
    if (this.props.open) {
      this.handleClose();
    }
  }

  dialogEl = (): Element | null => {
    if (!this.modalEl) {
      return null;
    }

    const isBackdropRenderedFirst = !this.props.hideBackdrop;

    if (isBackdropRenderedFirst) {
      return this.modalEl.children[1];
    }

    return this.modalEl.firstElementChild;
  };

  handleOpen = () => {
    document.addEventListener('keydown', this.handleDocumentKeyDown);
    this.enableFocusTrap();

    if (this.dialogEl()) {
      this.handleOpened();
    }
  };

  handleOpened = () => {
    // Fix a bug on Chrome where the scroll isn't initially 0.
    if (this.modalEl) {
      this.modalEl.scrollTop = 0;
    }
  };

  handleClose = () => {
    document.removeEventListener('keydown', this.handleDocumentKeyDown);
    this.disableFocusTrap();

    this.restoreLastFocus();
  };

  handleBackdropClick = (event: React.MouseEvent) => {
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

  handleDocumentKeyDown = (event: KeyboardEvent) => {
    if (event.key === 'Tab') {
      this.handleFocusTrapTab(event);
    }

    // Ignore events that have been `event.preventDefault()` marked.
    if (event.key !== 'Escape' || event.defaultPrevented) {
      return;
    }

    if (this.props.onEscapeKeyDown) {
      this.props.onEscapeKeyDown(event);
    }

    if (!this.props.disableEscapeKeyDown && this.props.onClose) {
      this.props.onClose(event, 'escapeKeyDown');
    }
  };

  // Focus trap methods.
  //
  // When trapFocus is enabled, focus is kept inside the modal. Programmatic focus theft from
  // outside (e.g., useRefAutoFocus on a Document) is reverted to the last focused element inside
  // the modal. Tab/Shift+Tab cycling wraps around at the modal boundaries.

  enableFocusTrap = () => {
    if (!this.props.trapFocus) {
      return;
    }

    // If focus is already inside the modal (e.g., from autoFocus on a button during render),
    // remember it before registering the listener, so that programmatic focus theft can be
    // reverted.
    const activeEl = document.activeElement as HTMLElement;
    if (activeEl && this.modalEl?.contains(activeEl)) {
      this.lastModalFocus = activeEl;
    }
    document.addEventListener('focusin', this.handleFocusTrapFocusIn);
  };

  disableFocusTrap = () => {
    document.removeEventListener('focusin', this.handleFocusTrapFocusIn);
    this.lastModalFocus = undefined;
    this.lastTabKeyDirection = null;
  };

  handleFocusTrapFocusIn = (event: FocusEvent) => {
    if (!this.modalEl) {
      return;
    }

    const target = event.target as HTMLElement;

    if (this.modalEl.contains(target)) {
      this.lastModalFocus = target;
    } else if (this.lastModalFocus) {
      // Focus escaped the modal — pull it back to the last focused element.
      this.lastModalFocus.focus();
    } else if (this.lastTabKeyDirection) {
      // Tab/Shift+Tab moved focus outside the modal before anything inside was focused.
      // Direct focus to the appropriate element inside the modal.
      const focusable = this.getFocusableElements();
      const el =
        this.lastTabKeyDirection === 'forward'
          ? focusable[0]
          : focusable[focusable.length - 1];
      el?.focus();
    } else {
      // Programmatic focus theft (Tab was not pressed) with nothing focused inside yet — just blur
      // the thief.
      target.blur();
    }
    this.lastTabKeyDirection = null;
  };

  handleFocusTrapTab = (event: KeyboardEvent) => {
    if (!this.props.trapFocus || !this.modalEl) {
      return;
    }

    // Record the Tab direction so that handleFocusTrapFocusIn can tell a Tab keypress apart from
    // programmatic focus theft.
    this.lastTabKeyDirection = event.shiftKey ? 'backward' : 'forward';

    const focusable = this.getFocusableElements();
    if (focusable.length === 0) {
      return;
    }

    const first = focusable[0];
    const last = focusable[focusable.length - 1];

    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  };

  /** Return focusable elements within the modal that are visible and not disabled. */
  getFocusableElements = (): HTMLElement[] => {
    if (!this.modalEl) {
      return [];
    }

    return [
      ...this.modalEl.querySelectorAll<HTMLElement>(
        'button:not([disabled]), [href], input:not([disabled]):not([type="hidden"]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
      ),
    ].filter(
      el =>
        el.offsetParent !== null && getComputedStyle(el).visibility !== 'hidden'
    );
  };

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

    this.lastFocus = undefined;
  }

  render() {
    const {
      BackdropProps,
      children,
      modalCss,
      hideBackdrop,
      open,
      className,
      keepInDOMAfterClose,
    } = this.props;

    if (!open && !keepInDOMAfterClose) {
      return null;
    }

    return createPortal(
      <StyledModal
        hiddenInDom={!open}
        modalCss={modalCss}
        data-testid="Modal"
        ref={el => {
          this.modalEl = el;
          const { modalRef } = this.props;
          if (modalRef) {
            if (typeof modalRef === 'function') {
              modalRef(el);
            } else {
              (modalRef as RefObject<HTMLDivElement | null>).current = el;
            }
          }
        }}
        className={className}
        onClick={e => e.stopPropagation()}
      >
        {!hideBackdrop && (
          <Backdrop onClick={this.handleBackdropClick} {...BackdropProps} />
        )}
        {children && cloneElement(children, {})}
      </StyledModal>,
      document.body
    );
  }
}

export type BackdropProps = {
  /**
   * Allows backdrop to keep bg color of parent eg: popup menu
   */
  invisible?: boolean;
  [prop: string]: any;
};

function Backdrop(props: BackdropProps) {
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

const StyledBackdrop = styled.div<BackdropProps>`
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

const StyledModal = styled.div<{
  modalCss?: StyleFunction<ComponentProps<'div'>>;
  hiddenInDom?: boolean;
}>`
  position: fixed;
  z-index: 1200;
  right: 0;
  bottom: 0;
  top: 0;
  left: 0;
  ${props => props.hiddenInDom && `display: none;`};
  ${props => props.modalCss?.(props)}
`;
