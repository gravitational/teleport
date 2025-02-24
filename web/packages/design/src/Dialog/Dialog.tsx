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

import { ComponentProps, forwardRef, PropsWithChildren } from 'react';
import styled, { StyleFunction } from 'styled-components';

import Modal, { ModalProps } from './../Modal';

// ModalProps enforces its own requirements with regards to the children prop through types
// which we do not care about here, as children passed to <Dialog> are not passed directly to <Modal>.
type ModalPropsWithoutChildren = Omit<ModalProps, 'children'>;

export const Dialog = forwardRef<
  HTMLDivElement,
  PropsWithChildren<
    {
      className?: string;
      dialogCss?: StyleFunction<ComponentProps<'div'>>;
    } & ModalPropsWithoutChildren
  >
>((props, ref) => {
  const { children, dialogCss, className, ...modalProps } = props;
  return (
    <Modal disableBackdropClick disableEscapeKeyDown {...modalProps}>
      <ModalBox>
        {/*
            ref is supposed to be set on DialogBox, not Modal, because when used with
            react-transition-group, it's DialogBox that's going to have its className set based on
            the transition state.
          */}
        <DialogBox
          ref={ref}
          data-testid="dialogbox"
          dialogCss={dialogCss}
          className={className}
        >
          {children}
        </DialogBox>
      </ModalBox>
    </Modal>
  );
});

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

const DialogBox = styled.div<{
  dialogCss: StyleFunction<ComponentProps<'div'>> | undefined;
}>`
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
  ${props => props.dialogCss?.(props)};
`;
