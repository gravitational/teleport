/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
