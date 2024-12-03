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

import { ComponentProps, ReactNode } from 'react';
import { StyleFunction } from 'styled-components';

import Dialog from 'design/Dialog';

export function DialogConfirmation(props: {
  open: boolean;
  /**
   * Prevent unmounting the component and its children from the DOM when closed.
   * Instead, hides it with CSS.
   */
  keepInDOMAfterClose?: boolean;
  /** @deprecated This props has no effect, it was never passed down to `Dialog`. */
  disableEscapeKeyDown?: boolean;
  children?: ReactNode;
  onClose?: (
    event: KeyboardEvent | React.MouseEvent,
    reason: 'escapeKeyDown' | 'backdropClick'
  ) => void;
  dialogCss?: StyleFunction<ComponentProps<'div'>>;
}) {
  return (
    <Dialog
      dialogCss={props.dialogCss}
      disableEscapeKeyDown={false}
      onClose={props.onClose}
      open={props.open}
      keepInDOMAfterClose={props.keepInDOMAfterClose}
    >
      {props.children}
    </Dialog>
  );
}
