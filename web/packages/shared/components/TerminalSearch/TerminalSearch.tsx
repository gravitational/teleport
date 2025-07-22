/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { SearchAddon } from '@xterm/addon-search';
import { useCallback, useEffect, useRef, useState } from 'react';
import styled, { useTheme } from 'styled-components';

import { Box, ButtonIcon, Flex, Input, P2 } from 'design';
import { ChevronDown, ChevronUp, Cross } from 'design/Icon';

export interface TerminalSearcher {
  getSearchAddon(): SearchAddon;
  focus(): void;
  registerCustomKeyEventHandler(customEvent: (e: KeyboardEvent) => boolean): {
    unregister(): void;
  };
}

export const TerminalSearch = ({
  terminalSearcher,
  show,
  onClose,
  onOpen,
  isSearchKeyboardEvent,
}: {
  terminalSearcher: TerminalSearcher;
  show: boolean;
  onClose(): void;
  onOpen(): void;
  isSearchKeyboardEvent(e: KeyboardEvent): boolean;
}) => {
  const theme = useTheme();
  const searchInputRef = useRef<HTMLInputElement>(null);
  const [searchValue, setSearchValue] = useState('');
  const [searchResults, setSearchResults] = useState<{
    resultIndex: number;
    resultCount: number;
  }>({ resultIndex: 0, resultCount: 0 });

  useEffect(() => {
    terminalSearcher.getSearchAddon().onDidChangeResults(setSearchResults);
  }, [terminalSearcher]);

  const search = (value: string, direction: 'next' | 'previous') => {
    const match = theme.colors.terminal.searchMatch;
    const activeMatch = theme.colors.terminal.activeSearchMatch;
    setSearchValue(value);
    const opts = {
      regex: true,
      caseSensitive: false,
      decorations: {
        matchOverviewRuler: match,
        activeMatchColorOverviewRuler: activeMatch,
        matchBackground: match,
        activeMatchBackground: activeMatch,
      },
    };

    if (direction === 'next') {
      terminalSearcher.getSearchAddon().findNext(value, opts);
    } else {
      terminalSearcher.getSearchAddon().findPrevious(value, opts);
    }
  };

  const onChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    search(e.target.value, 'next');
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      search(e.currentTarget.value, 'next');
    }
    // this is if you want to close the search bar with the search input focused
    if (e.key === 'Escape') {
      onEscape();
      e.preventDefault();
    }
  };

  const searchNext = () => {
    search(searchInputRef.current.value, 'next');
  };

  const searchPrevious = () => {
    search(searchInputRef.current.value, 'previous');
  };

  const onEscape = useCallback(() => {
    onClose();
    terminalSearcher.getSearchAddon().clearDecorations();
    terminalSearcher.focus();
  }, [onClose, terminalSearcher]);

  useEffect(() => {
    const { unregister } = terminalSearcher.registerCustomKeyEventHandler(e => {
      if (isSearchKeyboardEvent(e)) {
        onOpen();
        searchInputRef.current?.focus();
        e.preventDefault();
        // event was handled and does not need to be handled by xterm
        return false;
      }

      // continue through to xterm
      return true;
    });

    return unregister;
  }, [onOpen, terminalSearcher, isSearchKeyboardEvent]);

  if (!show) {
    return;
  }

  const hasResults = searchResults.resultCount > 0;

  return (
    <SearchInputContainer alignItems="center" gap={3}>
      <Input
        ref={searchInputRef}
        data-testid="terminal-search"
        size="small"
        autoFocus
        onFocus={e => e.target.select()}
        value={searchValue}
        onChange={onChange}
        onKeyDown={onKeyDown}
        width={150}
      />
      <Box minWidth={45}>
        <P2>{`${searchResults.resultCount === 0 ? 0 : searchResults.resultIndex + 1}/${searchResults.resultCount}`}</P2>
      </Box>
      <Flex gap={1}>
        <SearchButtonIcon
          disabled={!hasResults}
          title="Search previous"
          onClick={searchPrevious}
        >
          <ChevronUp size="medium" />
        </SearchButtonIcon>
        <SearchButtonIcon
          disabled={!hasResults}
          title="Search next"
          onClick={searchNext}
        >
          <ChevronDown size="medium" />
        </SearchButtonIcon>
        <SearchButtonIcon title="Close search" onClick={onEscape}>
          <Cross size="medium" />
        </SearchButtonIcon>
      </Flex>
    </SearchInputContainer>
  );
};

const SearchInputContainer = styled(Flex)`
  padding: ${p => p.theme.space[2]}px;
  padding-left: ${p => p.theme.space[3]}px;
  padding-right: ${p => p.theme.space[3]}px;
  background-color: ${p => p.theme.colors.levels.surface};
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
  border-radius: ${props => props.theme.radii[2]}px;
`;

const SearchButtonIcon = styled(ButtonIcon)`
  border-radius: ${props => props.theme.radii[2]}px;
`;
