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
import React, { useEffect, useRef } from 'react';

import { getPlatform } from 'design/theme/utils';
import { useTheme } from 'styled-components';

import Terminal from 'teleport/lib/term/terminal';
import Tty from 'teleport/lib/term/tty';
import { TermEvent } from 'teleport/lib/term/enums';
import StyledXterm from 'teleport/Console/StyledXterm';

export default function Xterm({ tty }: { tty: Tty }) {
  const refContainer = useRef<HTMLElement>();
  const theme = useTheme();
  const terminalPlayer = useRef<TerminalPlayer>();

  useEffect(() => {
    const term = new TerminalPlayer(tty, {
      el: refContainer.current,
      fontFamily: theme.fonts.mono,
      fontSize: getPlatform().isMac ? 12 : 14,
      theme: theme.colors.terminal,
    });

    terminalPlayer.current = term;
    term.open();
    term.term.focus();

    term.tty.on(TermEvent.DATA, () => {
      // Keeps the cursor in view.
      term.term.textarea.scrollIntoView(false);
    });

    function stopPropagating(e: Event) {
      e.stopPropagation();
    }

    // Stop wheel event from reaching the terminal
    // to allow parent container of xterm to scroll instead.
    window.addEventListener('wheel', stopPropagating, true);

    function cleanup() {
      term.destroy();
      window.removeEventListener('wheel', stopPropagating, true);
    }

    return cleanup;
    // do not re-initialize xterm when theme changes, use specialized handlers.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tty]);

  useEffect(() => {
    terminalPlayer.current?.updateTheme(theme.colors.terminal);
  }, [theme]);

  return <StyledXterm ref={refContainer} />;
}

class TerminalPlayer extends Terminal {
  // do not attempt to connect
  connect() {
    // Prevents terminal scrolling to force users to rely on the
    // player controls.
    this.term.options.scrollback = 0;
  }

  resize(cols, rows) {
    // ensure that cursor is visible as xterm hides it on blur event
    this.term.focus();
    super.resize(cols, rows);
  }

  // prevent user resize requests
  _requestResize() {}
}
