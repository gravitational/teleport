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
  useState,
} from 'react';
import { Flex } from 'design';
import { ITheme } from 'xterm';

import { getPlatformType } from 'design/platform';

import Tty from 'teleport/lib/term/tty';
import XTermCtrl from 'teleport/lib/term/terminal';
import { getMappedAction } from 'teleport/Console/useKeyboardNav';

import { TerminalAssist } from 'teleport/Console/DocumentSsh/TerminalAssist/TerminalAssist';
import { ActionBar } from 'teleport/Console/DocumentSsh/TerminalAssist/ActionBar';
import { useTerminalAssist } from 'teleport/Console/DocumentSsh/TerminalAssist/TerminalAssistContext';

import StyledXterm from '../../StyledXterm';

export interface TerminalRef {
  focus(): void;
}

export interface TerminalProps {
  tty: Tty;
  fontFamily: string;
  theme: ITheme;
  assistEnabled: boolean;
}

interface ActionBarState {
  visible: boolean;
  left: number;
  top: number;
}

export const Terminal = forwardRef<TerminalRef, TerminalProps>((props, ref) => {
  const termCtrlRef = useRef<XTermCtrl>();
  const elementRef = useRef<HTMLElement>();

  const assist = useTerminalAssist();

  const [actionBarState, setActionBarState] = useState<ActionBarState | null>({
    top: 0,
    left: 0,
    visible: false,
  });

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
    });
    termCtrlRef.current = termCtrl;

    termCtrl.open();

    termCtrl.term.attachCustomKeyEventHandler(event => {
      const { tabSwitch } = getMappedAction(event);
      if (tabSwitch) {
        return false;
      }
    });

    if (props.assistEnabled) {
      termCtrl.term.onSelectionChange(() => {
        const term = termCtrl.term;

        const position = term.getSelectionPosition();
        const selection = term.getSelection().trim();

        if (position && selection) {
          const charWidth = Math.ceil(term.element.offsetWidth / term.cols);
          const charHeight = Math.ceil(term.element.offsetHeight / term.rows);

          const left = Math.round(
            ((position.start.x + position.end.x) / 2) * charWidth
          );
          const top = Math.round((position.end.y + 2) * charHeight) + 15;

          setActionBarState({
            visible: true,
            left,
            top,
          });

          return;
        }

        setActionBarState(position => ({
          ...position,
          visible: false,
        }));
      });
    }

    return () => termCtrl.destroy();
    // do not re-initialize xterm when theme changes, use specialized handlers.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [props.assistEnabled]);

  function handleUseCommand(command: string) {
    termCtrlRef.current.term.paste(command);
  }

  function handleAssistClose() {
    termCtrlRef.current.term.focus();
  }

  function handleAskAssist() {
    assist.explainSelection(termCtrlRef.current.term.getSelection());
  }

  function handleActionBarClose() {
    setActionBarState(position => ({
      ...position,
      visible: false,
    }));
  }

  function handleActionBarCopy() {
    const selection = termCtrlRef.current.term.getSelection();

    if (selection) {
      void navigator.clipboard.writeText(selection);
    }
  }

  useEffect(() => {
    termCtrlRef.current?.updateTheme(props.theme);
  }, [props.theme]);

  return (
    <>
      <Flex
        flexDirection="column"
        height="100%"
        width="100%"
        px="2"
        style={{ overflow: 'auto' }}
      >
        <StyledXterm ref={elementRef} />
      </Flex>

      {props.assistEnabled && (
        <>
          <ActionBar
            position={actionBarState}
            visible={actionBarState.visible}
            onClose={handleActionBarClose}
            onCopy={handleActionBarCopy}
            onAskAssist={handleAskAssist}
          />

          <TerminalAssist
            onUseCommand={handleUseCommand}
            onClose={handleAssistClose}
          />
        </>
      )}
    </>
  );
});
