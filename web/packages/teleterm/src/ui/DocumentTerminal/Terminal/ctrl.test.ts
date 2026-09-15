/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import Logger, { NullService } from 'teleterm/logger';
import { createMockConfigService } from 'teleterm/services/config/fixtures/mocks';
import { MockPtyProcess } from 'teleterm/services/pty/fixtures/mocks';
import { KeyboardShortcutsService } from 'teleterm/ui/services/keyboardShortcuts';

import TtyTerminal from './ctrl';

beforeAll(() => {
  Logger.init(new NullService());
});

describe('Alt+Arrow sends escape sequences for word navigation', () => {
  it.each([
    // Escape codes don't render well in the terminal when running tests, hence we use arrays
    // instead of objects to be able to use %j in test name to automatically escape the 2nd arg.
    ['ArrowLeft', '\x1bb'],
    ['ArrowRight', '\x1bf'],
  ])('Alt+%s sends %j', (key, expectedSeq) => {
    const { tty, writeFn } = createTerminal();

    dispatchKeydown(tty, { key, altKey: true });

    expect(writeFn).toHaveBeenCalledWith(expectedSeq);
    tty.destroy();
  });

  it('does not intercept Ctrl+Alt+Arrow', () => {
    const { tty, writeFn } = createTerminal();

    dispatchKeydown(tty, { key: 'ArrowLeft', altKey: true, ctrlKey: true });

    // writeFn may be called by xterm's default handling, but not with our Alt+arrow sequences.
    expect(writeFn).not.toHaveBeenCalledWith('\x1bb');
    tty.destroy();
  });
});

// Without the Unicode graphemes addon, xterm.js measures these with Unicode 6
// width tables: ⚡, 🤖 and ❤️ advance the cursor one cell and the ZWJ family
// advances three. The addon clusters each of them into a single two-cell
// grapheme, matching what modern terminals draw, so the rest of the frame
// stops shifting.
describe('graphemes rendering', () => {
  it.each([
    ['⚡', 'U+26A1'],
    ['🤖', 'U+1F916'],
    ['❤️', 'U+2764 U+FE0F'],
    ['👨‍👩‍👧', 'ZWJ sequence'],
  ])('%s (%s) is two cells wide', async text => {
    const { tty } = createTerminal();

    await write(tty, text);

    expect(tty.term.buffer.active.cursorX).toBe(2);
    tty.destroy();
  });
});

function write(tty: TtyTerminal, data: string) {
  return new Promise<void>(resolve => tty.term.write(data, resolve));
}

function createTerminal() {
  const el = document.createElement('div');
  document.body.appendChild(el);

  const ptyProcess = new MockPtyProcess();
  const writeFn = jest.spyOn(ptyProcess, 'write');
  const configService = createMockConfigService({});
  const keyboardShortcutsService = new KeyboardShortcutsService(
    'darwin',
    configService
  );

  const tty = new TtyTerminal(
    ptyProcess,
    {
      el,
      fontSize: 12,
      theme: {},
      windowsPty: undefined,
      openContextMenu: jest.fn(),
    },
    configService,
    keyboardShortcutsService
  );
  tty.open();

  return { tty, writeFn, el };
}

function dispatchKeydown(
  tty: TtyTerminal,
  opts: { key: string; altKey?: boolean; ctrlKey?: boolean }
) {
  const event = new KeyboardEvent('keydown', {
    key: opts.key,
    code: opts.key,
    altKey: opts.altKey ?? false,
    ctrlKey: opts.ctrlKey ?? false,
  });
  tty.term.textarea.dispatchEvent(event);
}
