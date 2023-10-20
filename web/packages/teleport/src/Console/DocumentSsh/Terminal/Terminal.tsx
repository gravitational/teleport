/*
Copyright 2023 Gravitational, Inc.

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

import React, {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from 'react';
import { Flex } from 'design';
import { ITheme } from 'xterm';

import { getPlatform } from 'design/theme/utils';

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
    const platform = getPlatform();
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
