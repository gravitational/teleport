/**
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

import React, {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
} from 'react';
import { Flex } from 'design';
import { ITheme } from '@xterm/xterm';

import { getPlatformType } from 'design/platform';

import Tty from 'teleport/lib/term/tty';
import XTermCtrl from 'teleport/lib/term/terminal';
import { getMappedAction } from 'teleport/Console/useKeyboardNav';

import StyledXterm from '../../StyledXterm';

export interface TerminalRef {
  focus(): void;
}

export interface TerminalProps {
  tty: Tty;
  fontFamily: string;
  theme: ITheme;
  // convertEol when set to true cursor will be set to the beginning of the next line with every received new line symbol.
  // This is equivalent to replacing each '\n' with '\r\n'.
  convertEol?: boolean;
}

export const Terminal = forwardRef<TerminalRef, TerminalProps>((props, ref) => {
  const termCtrlRef = useRef<XTermCtrl>();
  const elementRef = useRef<HTMLDivElement>();

  useImperativeHandle(
    ref,
    () => ({
      focus: () => termCtrlRef.current.term.focus(),
    }),
    []
  );

  useEffect(() => {
    const platform = getPlatformType();
    const fontSize = platform.isMac ? 12 : 14;

    const termCtrl = new XTermCtrl(props.tty, {
      el: elementRef.current,
      fontFamily: props.fontFamily,
      fontSize,
      theme: props.theme,
      convertEol: props.convertEol,
    });
    termCtrlRef.current = termCtrl;

    termCtrl.open();

    termCtrl.term.attachCustomKeyEventHandler(event => {
      const { tabSwitch } = getMappedAction(event);
      if (tabSwitch) {
        return false;
      }
    });

    return () => termCtrl.destroy();
    // do not re-initialize xterm when theme changes, use specialized handlers.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    termCtrlRef.current?.updateTheme(props.theme);
  }, [props.theme]);

  return (
    <Flex
      flexDirection="column"
      height="100%"
      width="100%"
      px="2"
      style={{ overflow: 'auto' }}
      data-testid="terminal"
    >
      <StyledXterm ref={elementRef} />
    </Flex>
  );
});
