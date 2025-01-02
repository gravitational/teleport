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

import { useCallback, useEffect, useRef, useState } from 'react';
import styled, { useTheme } from 'styled-components';

import { getPlatformType } from 'design/platform';
import { TerminalSearch } from 'shared/components/TerminalSearch';

import StyledXterm from 'teleport/Console/StyledXterm';
import { TermEvent } from 'teleport/lib/term/enums';
import Terminal from 'teleport/lib/term/terminal';
import Tty from 'teleport/lib/term/tty';

export default function Xterm({ tty }: { tty: Tty }) {
  const refContainer = useRef<HTMLDivElement>();
  const theme = useTheme();
  const terminalPlayer = useRef<TerminalPlayer>();
  const [showSearch, setShowSearch] = useState(false);

  const onSearchClose = useCallback(() => {
    setShowSearch(false);
  }, []);

  const onSearchOpen = useCallback(() => {
    setShowSearch(true);
  }, []);

  const isSearchKeyboardEvent = useCallback((e: KeyboardEvent) => {
    return (e.metaKey || e.ctrlKey) && e.key === 'f';
  }, []);

  useEffect(() => {
    const term = new TerminalPlayer(tty, {
      el: refContainer.current,
      fontFamily: theme.fonts.mono,
      fontSize: getPlatformType().isMac ? 12 : 14,
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

  return (
    <>
      <StyledXterm ref={refContainer} />
      {terminalPlayer.current && (
        <TerminalAddonsContainer>
          <TerminalSearch
            show={showSearch}
            isSearchKeyboardEvent={isSearchKeyboardEvent}
            onClose={onSearchClose}
            onOpen={onSearchOpen}
            terminalSearcher={terminalPlayer.current}
          />
        </TerminalAddonsContainer>
      )}
    </>
  );
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

const TerminalAddonsContainer = styled.div`
  position: absolute;
  right: ${p => p.theme.space[2]}px;
  top: ${p => p.theme.space[2]}px;
  z-index: 10;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: ${p => p.theme.space[2]}px;
  min-width: 500px;
`;
