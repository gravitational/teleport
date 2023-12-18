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

import { makeRuntimeSettings } from 'teleterm/mainProcess/fixtures/mocks';

import { ShellCommand, GatewayCliClientCommand } from '../types';

import { getPtyProcessOptions } from './buildPtyOptions';

describe('getPtyProcessOptions', () => {
  describe('pty.gateway-cli-client', () => {
    it('merges process env with the env from cmd', () => {
      const processEnv = {
        processExclusive: 'process',
        shared: 'fromProcess',
      };
      const cmd: GatewayCliClientCommand = {
        kind: 'pty.gateway-cli-client',
        path: 'foo',
        args: [],
        clusterName: 'bar',
        proxyHost: 'baz',
        env: {
          cmdExclusive: 'cmd',
          shared: 'fromCmd',
        },
      };

      const { env } = getPtyProcessOptions(
        makeRuntimeSettings(),
        cmd,
        processEnv
      );

      expect(env.processExclusive).toBe('process');
      expect(env.cmdExclusive).toBe('cmd');
      expect(env.shared).toBe('fromCmd');
    });
  });

  describe('pty.shell', () => {
    it('merges process env with the env from cmd', () => {
      const processEnv = {
        processExclusive: 'process',
        shared: 'fromProcess',
      };
      const cmd: ShellCommand = {
        kind: 'pty.shell',
        clusterName: 'bar',
        proxyHost: 'baz',
        env: {
          cmdExclusive: 'cmd',
          shared: 'fromCmd',
        },
      };

      const { env } = getPtyProcessOptions(
        makeRuntimeSettings(),
        cmd,
        processEnv
      );

      expect(env.processExclusive).toBe('process');
      expect(env.cmdExclusive).toBe('cmd');
      expect(env.shared).toBe('fromCmd');
    });
  });
});
