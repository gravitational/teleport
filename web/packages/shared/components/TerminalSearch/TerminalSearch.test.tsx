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

import { act, render, screen } from 'design/utils/testing';

import { TerminalSearch } from './TerminalSearch';

let searchCallback: SearchCallbackType;
type SearchCallbackType = (results: {
  resultIndex: number;
  resultCount: number;
}) => void;

vi.mock('@xterm/addon-search', () => {
  const SearchAddon = vi.fn(function() {
    return {
      findNext: vi.fn(),
      findPrevious: vi.fn(),
      clearDecorations: vi.fn(),
      onDidChangeResults: vi.fn(callback => {
        searchCallback = callback;
        return { dispose: vi.fn() };
      }),
    };
  });
  return { SearchAddon };
});

const createTerminalMock = () => {
  const keyEventHandlers = new Set<(e: KeyboardEvent) => boolean>();

  return {
    getSearchAddon: () => new SearchAddon(),
    focus: vi.fn(),
    registerCustomKeyEventHandler: (handler: (e: KeyboardEvent) => boolean) => {
      keyEventHandlers.add(handler);
      return {
        unregister: () => keyEventHandlers.delete(handler),
      };
    },
    // Helper to simulate keyboard events
    triggerKeyEvent: (eventProps: Partial<KeyboardEvent>) => {
      const event = new KeyboardEvent('keydown', eventProps);
      keyEventHandlers.forEach(handler => handler(event));
    },
    // Helper to simulate search results
    triggerSearchResults: (resultIndex: number, resultCount: number) => {
      searchCallback?.({ resultIndex, resultCount });
    },
  };
};

const renderComponent = (props = {}) => {
  const terminalMock = createTerminalMock();
  const defaultProps = {
    terminalSearcher: terminalMock,
    show: true,
    onClose: vi.fn(),
    isSearchKeyboardEvent: vi.fn(),
    onOpen: vi.fn(),
    ...props,
  };

  return {
    ...render(<TerminalSearch {...defaultProps} />),
    terminalMock,
    props: defaultProps,
  };
};

const terminalSearchTestId = 'terminal-search';
const searchNext = /search next/i;
const searchPrevious = /search previous/i;
const closeSearch = /close search/i;

describe('TerminalSearch', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    searchCallback = null;
  });

  test('no render when show is false', () => {
    renderComponent({ show: false });
    expect(screen.queryByTestId(terminalSearchTestId)).not.toBeInTheDocument();
  });

  test('render search input and buttons when show is true', () => {
    renderComponent();
    expect(screen.getByTestId(terminalSearchTestId)).toBeInTheDocument();
    expect(screen.getByTitle(searchNext)).toBeInTheDocument();
    expect(screen.getByTitle(searchPrevious)).toBeInTheDocument();
    expect(screen.getByTitle(closeSearch)).toBeInTheDocument();
  });

  test('show initial search results as 0/0', () => {
    renderComponent();
    expect(screen.getByText('0/0')).toBeInTheDocument();
  });

  test('open search when Ctrl+F is pressed', () => {
    const isSearchKeyboardEvent = vi.fn().mockReturnValue(true);
    const { props, terminalMock } = renderComponent({ isSearchKeyboardEvent });

    terminalMock.triggerKeyEvent({
      key: 'f',
      ctrlKey: true,
      type: 'keydown',
    });

    expect(props.onOpen).toHaveBeenCalled();
  });

  test('open search when Cmd+F is pressed (Mac)', () => {
    const isSearchKeyboardEvent = vi.fn().mockReturnValue(true);
    const { props, terminalMock } = renderComponent({ isSearchKeyboardEvent });

    terminalMock.triggerKeyEvent({
      key: 'f',
      metaKey: true,
      type: 'keydown',
    });

    expect(props.onOpen).toHaveBeenCalled();
  });

  test('show result counts', async () => {
    const { terminalMock } = renderComponent();

    const testCases = [
      { resultIndex: 0, resultCount: 1, expected: '1/1' },
      { resultIndex: 1, resultCount: 3, expected: '2/3' },
      { resultIndex: 4, resultCount: 10, expected: '5/10' },
    ];

    for (const { resultIndex, resultCount, expected } of testCases) {
      await act(async () => {
        terminalMock.triggerSearchResults(resultIndex, resultCount);
      });
      expect(screen.getByText(expected)).toBeInTheDocument();
    }
  });
});
