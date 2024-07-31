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

import React, { useEffect, useRef, useState } from 'react';
import styled, { useTheme } from 'styled-components';
import { Box, Flex } from 'design';
import { debounce } from 'shared/utils/highbar';
import {
  Attempt,
  makeEmptyAttempt,
  makeErrorAttemptWithStatusText,
  makeSuccessAttempt,
} from 'shared/hooks/useAsync';

import { WindowsPty } from 'teleterm/services/pty';
import { IPtyProcess } from 'teleterm/sharedProcess/ptyHost';
import { DocumentTerminal } from 'teleterm/ui/services/workspacesService';
import { KeyboardShortcutsService } from 'teleterm/ui/services/keyboardShortcuts';

import { Reconnect } from '../Reconnect';

import XTermCtrl from './ctrl';

type TerminalProps = {
  docKind: DocumentTerminal['kind'];
  ptyProcess: IPtyProcess;
  reconnect: () => void;
  visible: boolean;
  /**
   * This value can be provided by the user and is unsanitized. This means that it cannot be directly interpolated
   * in a styled component or used in CSS, as it may inject malicious CSS code.
   * Before using it, sanitize it with `CSS.escape` or pass it as a `style` prop.
   * Read more https://frontarm.com/james-k-nelson/how-can-i-use-css-in-js-securely/.
   */
  unsanitizedFontFamily: string;
  fontSize: number;
  onEnterKey?(): void;
  windowsPty: WindowsPty;
  keyboardShortcutsService: KeyboardShortcutsService;
};

export function Terminal(props: TerminalProps) {
  const refElement = useRef<HTMLElement>();
  const refCtrl = useRef<XTermCtrl>();
  const [startPtyProcessAttempt, setStartPtyProcessAttempt] =
    useState<Attempt<void>>(makeEmptyAttempt());
  const theme = useTheme();

  useEffect(() => {
    const removeOnStartErrorListener = props.ptyProcess.onStartError(
      message => {
        setStartPtyProcessAttempt(makeErrorAttemptWithStatusText(message));
      }
    );

    const removeOnOpenListener = props.ptyProcess.onOpen(() => {
      setStartPtyProcessAttempt(makeSuccessAttempt(undefined));
    });

    const ctrl = new XTermCtrl(
      props.ptyProcess,
      {
        el: refElement.current,
        fontSize: props.fontSize,
        theme: theme.colors.terminal,
        windowsPty: props.windowsPty,
      },
      props.keyboardShortcutsService
    );

    // Start the PTY process.
    ctrl.open();

    ctrl.term.onKey(event => {
      if (event.domEvent.key === 'Enter') {
        handleEnterPress();
      }
    });

    refCtrl.current = ctrl;

    const handleEnterPress = debounce(() => {
      props.onEnterKey && props.onEnterKey();
    }, 100);

    return () => {
      removeOnStartErrorListener();
      removeOnOpenListener();
      handleEnterPress.cancel();
      ctrl.destroy();
    };
  }, []);

  useEffect(() => {
    if (!refCtrl.current || !props.visible) {
      return;
    }

    refCtrl.current.focus();
    refCtrl.current.requestResize();
  }, [props.visible]);

  useEffect(() => {
    if (refCtrl.current) {
      refCtrl.current.term.options.theme = theme.colors.terminal;
    }
  }, [theme]);

  return (
    <Flex
      flexDirection="column"
      height="100%"
      width="100%"
      style={{ overflow: 'hidden' }}
    >
      {startPtyProcessAttempt.status === 'error' && (
        <Reconnect
          docKind={props.docKind}
          attempt={startPtyProcessAttempt}
          reconnect={props.reconnect}
        />
      )}
      <StyledXterm
        ref={refElement}
        style={{
          fontFamily: props.unsanitizedFontFamily,
          // Always render the Xterm element so that refElement is not undefined on startError.
          display: startPtyProcessAttempt.status === 'error' ? 'none' : 'block',
        }}
      />
    </Flex>
  );
}

const StyledXterm = styled(Box)`
  height: 100%;
  width: 100%;
  background-color: ${props => props.theme.colors.levels.sunken};
`;
