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

import 'jest-canvas-mock';

import TtyTerminal from './terminal';
import Tty from './tty';

jest.mock('./tty');

describe('Alt+Arrow sends escape sequences for word navigation', () => {
  it.each([
    // Escape codes don't render well in the terminal when running tests, hence we use arrays
    // instead of objects to be able to use %j in test name to automatically escape the 2nd arg.
    ['ArrowLeft', '\x1bb'],
    ['ArrowRight', '\x1bf'],
  ])('Alt+%s sends %j', (key, expectedSeq) => {
    const { terminal, sendFn } = createTerminal();

    dispatchKeydown(terminal, { key, altKey: true });

    expect(sendFn).toHaveBeenCalledWith(expectedSeq);
    terminal.destroy();
  });

  it('does not intercept Ctrl+Alt+Arrow', () => {
    const { terminal, sendFn } = createTerminal();

    dispatchKeydown(terminal, {
      key: 'ArrowLeft',
      altKey: true,
      ctrlKey: true,
    });

    // sendFn may be called by xterm's default handling, but not with our Alt+arrow sequences.
    expect(sendFn).not.toHaveBeenCalledWith('\x1bb');
    terminal.destroy();
  });
});

function createTerminal() {
  const el = document.createElement('div');
  document.body.appendChild(el);

  const tty = new Tty(undefined) as jest.Mocked<Tty>;
  const sendFn = tty.send;

  const terminal = new TtyTerminal(tty, {
    el,
    theme: {},
  });
  terminal.open();

  return { terminal, sendFn, el };
}

function dispatchKeydown(
  terminal: TtyTerminal,
  opts: { key: string; altKey?: boolean; ctrlKey?: boolean }
) {
  const event = new KeyboardEvent('keydown', {
    key: opts.key,
    code: opts.key,
    altKey: opts.altKey ?? false,
    ctrlKey: opts.ctrlKey ?? false,
  });
  terminal.term.textarea.dispatchEvent(event);
}
