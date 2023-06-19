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
} from 'react';
import { Flex } from 'design';

import { getPlatform } from 'design/theme/utils';

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
}

export const Terminal = forwardRef<TerminalRef, TerminalProps>((props, ref) => {
  const termCtrlRef = useRef<XTermCtrl>();
  const elementRef = useRef<HTMLElement>();

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

  return (
    <Flex
      flexDirection="column"
      height="100%"
      width="100%"
      px="2"
      style={{ overflow: 'auto' }}
    >
      <StyledXterm ref={elementRef} />
    </Flex>
  );
});
