/**
 * @jest-environment node
 */
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

import crypto from 'node:crypto';

import Logger, { NullService } from 'teleterm/logger';

import { PtyProcess } from './ptyProcess';

beforeAll(() => {
  Logger.init(new NullService());
});

describe('PtyProcess', () => {
  describe('start', () => {
    it('emits a start error when attempting to execute a nonexistent command', async () => {
      const path = `nonexistent-executable-${crypto.randomUUID()}`;
      const pty = new PtyProcess({
        path,
        args: [],
        env: { PATH: '/foo/bar' },
        ptyId: '1234',
        useConpty: true,
      });

      const startErrorCb = jest.fn();

      pty.onStartError(startErrorCb);

      await pty.start(80, 20);

      expect(startErrorCb).toHaveBeenCalledWith(
        expect.stringContaining(`not found: ${path}`)
      );
    });
  });
});
