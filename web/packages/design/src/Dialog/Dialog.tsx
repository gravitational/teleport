/**
 Copyright 2019-2023 Gravitational, Inc.

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

import React, { CSSProperties, ReactNode, useEffect, useRef } from 'react';
import styled from 'styled-components';

export interface DialogProps {
  /**
   * A node, component instance, or function that returns either.
   * The `container` will have the portal children appended to it.
   */
  children?: ReactNode;
  /** If `true`, the modal is open. */
  open: boolean;
  className?: string;
  /** If `true`, modal will not close when backdrop is clicked.*/
  disableBackdropClick?: boolean;
  /** If `true`, modal will not close when escape is pressed. */
  disableEscapeKeyDown?: boolean;
  invisibleBackdrop?: boolean;

  onClose?(): void;

  dialogCss?(props: DialogProps): CSSProperties | string;
}

export function Dialog({
  disableBackdropClick = true,
  disableEscapeKeyDown = true,
  ...otherProps
}: DialogProps) {
  const dialogRef = useRef<HTMLDialogElement>();

  useEffect(() => {
    const currentModalRef = dialogRef.current;
    if (otherProps.open && !currentModalRef?.open) {
      currentModalRef?.showModal();
    }
    if (!otherProps.open && currentModalRef?.open) {
      currentModalRef?.close();
    }
  }, [otherProps.open]);

  function handleCancel(event: Event): void {
    if (disableEscapeKeyDown) {
      event.preventDefault();
    }
  }

  function handleClick(event: Event): void {
    if (disableBackdropClick || !otherProps.onClose) {
      return;
    }
    if (event.target === dialogRef.current) {
      otherProps.onClose();
    }
  }

  if (!otherProps.open) {
    return null;
  }

  return (
    <StyledDialog
      ref={dialogRef}
      className={otherProps.className}
      invisibleBackdrop={otherProps.invisibleBackdrop}
      data-testid="dialogbox"
      onClose={otherProps.onClose}
      onCancel={handleCancel}
      onClick={handleClick}
    >
      {otherProps.children}
    </StyledDialog>
  );
}

const StyledDialog = styled.dialog`
  border: none;
  background: none;
  padding: 24px 32px 32px;
  transition: opacity 225ms cubic-bezier(0.4, 0, 0.2, 1) 0ms;
  background: ${props => props.theme.colors.primary.main};
  color: ${props => props.theme.colors.text.primary};
  border-radius: 8px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.24);
  max-height: calc(100% - 96px);

  &[open] {
    display: flex;
    flex-direction: column;
  }

  ::backdrop {
    background-color: ${props =>
      props.invisibleBackdrop ? 'transparent' : 'rgba(0, 0, 0, 0.5)'};
  }

  ${props => props.dialogCss && props.dialogCss(props)};
`;
