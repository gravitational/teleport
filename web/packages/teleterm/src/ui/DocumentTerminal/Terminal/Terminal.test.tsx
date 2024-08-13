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

import 'jest-canvas-mock';
import { EventEmitter } from 'node:events';

import userEvent from '@testing-library/user-event';
import { screen, waitFor } from '@testing-library/react';
import { render } from 'design/utils/testing';

import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import Logger, { NullService } from 'teleterm/logger';
import { IAppContext } from 'teleterm/ui/types';

import { Terminal } from './Terminal';

beforeAll(() => {
  Logger.init(new NullService());
});

beforeEach(() => {
  userEvent.setup();
});

// TODO(gzdunek): Add tests for copying text.
// Unfortunately, simulating text selection with a single right click or
// mouseMove doesn't work.
// Probably because xterm doesn't render properly in JSDOM?
// I can see that the element with `xterm-screen` class has zero width and height.

test('keyboard shortcut pastes text', async () => {
  const appContext = new MockAppContext({ platform: 'win32' });

  render(<ConfiguredTerminal appContext={appContext} />);

  await navigator.clipboard.writeText('some-command');
  await userEvent.keyboard('{Control>}{Shift>}V'); // Ctrl+Shift+V

  await waitFor(() => {
    expect(screen.getByText('some-command')).toBeInTheDocument();
  });
});

test.each([
  {
    name: "mouse right click pastes text when 'terminal.rightClick: paste' is configured",
    getAppContext: () => {
      const appContext = new MockAppContext();
      appContext.configService.set('terminal.rightClick', 'paste');
      return appContext;
    },
  },
  {
    name: "mouse right click pastes text when 'terminal.rightClick: copyPaste' is configured",
    getAppContext: () => {
      const appContext = new MockAppContext();
      appContext.configService.set('terminal.rightClick', 'copyPaste');
      return appContext;
    },
  },
])(`$name`, async testCase => {
  const appContext = testCase.getAppContext();

  render(<ConfiguredTerminal appContext={appContext} />);

  await userEvent.keyboard('some-command ');
  const terminalContent = await screen.findByText('some-command');

  await navigator.clipboard.writeText('--flag=test');
  await userEvent.pointer({ keys: '[MouseRight>]', target: terminalContent });

  await waitFor(() => {
    expect(screen.getByText('some-command --flag=test')).toBeInTheDocument();
  });
});

test("mouse right click opens context menu when 'terminal.rightClick: menu' is configured", async () => {
  const appContext = new MockAppContext();
  jest.spyOn(appContext.mainProcessClient, 'openTerminalContextMenu');
  appContext.configService.set('terminal.rightClick', 'menu');
  const openContextMenu = jest.fn();

  render(
    <ConfiguredTerminal
      appContext={appContext}
      onOpenContextMenu={openContextMenu}
    />
  );

  await userEvent.keyboard('some-command ');
  const terminalContent = await screen.findByText('some-command');

  await navigator.clipboard.writeText('--flag=test');
  await userEvent.pointer({ keys: '[MouseRight>]', target: terminalContent });

  expect(openContextMenu).toHaveBeenCalledTimes(1);
  expect(openContextMenu).toHaveBeenCalledWith(
    expect.objectContaining({ defaultPrevented: true })
  );
});

function ConfiguredTerminal(props: {
  appContext: IAppContext;
  onOpenContextMenu?(): void;
}) {
  const emitter = new EventEmitter();
  const writeFn = jest.fn().mockImplementation(a => {
    emitter.emit('', a);
  });
  return (
    <Terminal
      docKind="doc.terminal_shell"
      ptyProcess={{
        start: () => '',
        write: writeFn,
        getPtyId: () => '',
        dispose: async () => {},
        getCwd: async () => '',
        onData: handler => {
          const listener = event => handler(event);
          emitter.addListener('', listener);
          return () => emitter.removeListener('', listener);
        },
        onExit: () => () => {},
        onOpen: () => () => {},
        onStartError: () => () => {},
        resize: () => {},
      }}
      reconnect={() => {}}
      visible={true}
      unsanitizedFontFamily={'monospace'}
      fontSize={12}
      openContextMenu={props.onOpenContextMenu}
      windowsPty={undefined}
      configService={props.appContext.configService}
      keyboardShortcutsService={props.appContext.keyboardShortcutsService}
    />
  );
}
