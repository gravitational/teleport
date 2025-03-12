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

import Logger, { NullService } from 'teleterm/logger';
import { makeRuntimeSettings } from 'teleterm/mainProcess/fixtures/mocks';

import {
  GatewayCliClientCommand,
  PtyProcessCreationStatus,
  ShellCommand,
  SshOptions,
  TshLoginCommand,
} from '../types';
import { buildPtyOptions, getPtyProcessOptions } from './buildPtyOptions';

beforeAll(() => {
  Logger.init(new NullService());
});

jest.mock('./resolveShellEnv', () => ({
  resolveShellEnvCached: () => Promise.resolve({}),
}));

const makeSshOptions = (options: Partial<SshOptions> = {}): SshOptions => ({
  noResume: false,
  forwardAgent: false,
  ...options,
});

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

      const { env } = getPtyProcessOptions({
        settings: makeRuntimeSettings(),
        options: {
          customShellPath: '',
          ssh: makeSshOptions(),
          windowsPty: { useConpty: true },
        },
        cmd: cmd,
        env: processEnv,
        shellBinPath: '/bin/zsh',
      });

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
        shellId: 'zsh',
        env: {
          cmdExclusive: 'cmd',
          shared: 'fromCmd',
        },
      };

      const { env } = getPtyProcessOptions({
        settings: makeRuntimeSettings(),
        options: {
          customShellPath: '',
          ssh: makeSshOptions(),
          windowsPty: { useConpty: true },
        },
        cmd: cmd,
        env: processEnv,
        shellBinPath: '/bin/zsh',
      });

      expect(env.processExclusive).toBe('process');
      expect(env.cmdExclusive).toBe('cmd');
      expect(env.shared).toBe('fromCmd');
    });

    it('disables SSH connection resumption on tsh ssh invocations if the option is set', () => {
      const processEnv = {
        processExclusive: 'process',
        shared: 'fromProcess',
      };
      const cmd: TshLoginCommand = {
        kind: 'pty.tsh-login',
        clusterName: 'bar',
        proxyHost: 'baz',
        login: 'bob',
        serverId: '01234567-89ab-cdef-0123-456789abcdef',
        rootClusterId: 'baz',
        leafClusterId: undefined,
      };

      const { args } = getPtyProcessOptions({
        settings: makeRuntimeSettings(),
        options: {
          customShellPath: '',
          ssh: makeSshOptions({ noResume: true }),
          windowsPty: { useConpty: true },
        },
        cmd: cmd,
        env: processEnv,
        shellBinPath: '/bin/zsh',
      });

      expect(args).toContain('--no-resume');
    });

    it('enables agent forwarding on tsh ssh invocations if the option is set', () => {
      const processEnv = {
        processExclusive: 'process',
        shared: 'fromProcess',
      };
      const cmd: TshLoginCommand = {
        kind: 'pty.tsh-login',
        clusterName: 'bar',
        proxyHost: 'baz',
        login: 'bob',
        serverId: '01234567-89ab-cdef-0123-456789abcdef',
        rootClusterId: 'baz',
        leafClusterId: undefined,
      };

      const { args } = getPtyProcessOptions({
        settings: makeRuntimeSettings(),
        options: {
          customShellPath: '',
          ssh: makeSshOptions({ forwardAgent: true }),
          windowsPty: { useConpty: true },
        },
        cmd: cmd,
        env: processEnv,
        shellBinPath: '/bin/zsh',
      });

      expect(args).toContain('--forward-agent');
    });

    it('does not enable agent forwarding on tsh ssh invocations if the option is not set', () => {
      const processEnv = {
        processExclusive: 'process',
        shared: 'fromProcess',
      };
      const cmd: TshLoginCommand = {
        kind: 'pty.tsh-login',
        clusterName: 'bar',
        proxyHost: 'baz',
        login: 'bob',
        serverId: '01234567-89ab-cdef-0123-456789abcdef',
        rootClusterId: 'baz',
        leafClusterId: undefined,
      };

      const { args } = getPtyProcessOptions({
        settings: makeRuntimeSettings(),
        options: {
          customShellPath: '',
          ssh: makeSshOptions({ forwardAgent: false }),
          windowsPty: { useConpty: true },
        },
        cmd: cmd,
        env: processEnv,
        shellBinPath: '/bin/zsh',
      });

      expect(args).not.toContain('--forward-agent');
    });
  });
});

describe('buildPtyOptions', () => {
  it('shellId is resolved to the shell object', async () => {
    const cmd: ShellCommand = {
      kind: 'pty.shell',
      clusterName: 'bar',
      proxyHost: 'baz',
      shellId: 'bash',
    };

    const { shell, creationStatus } = await buildPtyOptions({
      settings: makeRuntimeSettings({
        availableShells: [
          {
            id: 'bash',
            friendlyName: 'bash',
            binPath: '/bin/bash',
            binName: 'bash',
          },
        ],
      }),
      options: {
        customShellPath: '',
        ssh: makeSshOptions(),
        windowsPty: { useConpty: true },
      },
      cmd,
    });

    expect(shell).toEqual({
      id: 'bash',
      binPath: '/bin/bash',
      binName: 'bash',
      friendlyName: 'bash',
    });
    expect(creationStatus).toBe(PtyProcessCreationStatus.Ok);
  });

  it("custom shell path is resolved to the shell object when shellId is 'custom''", async () => {
    const cmd: ShellCommand = {
      kind: 'pty.shell',
      clusterName: 'bar',
      proxyHost: 'baz',
      shellId: 'custom',
    };

    const { shell, creationStatus } = await buildPtyOptions({
      settings: makeRuntimeSettings(),
      options: {
        customShellPath: '/custom/shell/path/better-shell',
        ssh: makeSshOptions(),
        windowsPty: { useConpty: true },
      },
      cmd,
    });

    expect(shell).toEqual({
      id: 'custom',
      binPath: '/custom/shell/path/better-shell',
      binName: 'better-shell',
      friendlyName: 'better-shell',
    });
    expect(creationStatus).toBe(PtyProcessCreationStatus.Ok);
  });

  it('if the provided shellId is not available, an OS default is returned', async () => {
    const cmd: ShellCommand = {
      kind: 'pty.shell',
      clusterName: 'bar',
      proxyHost: 'baz',
      shellId: 'no-such-shell',
    };

    const { shell, creationStatus } = await buildPtyOptions({
      settings: makeRuntimeSettings(),
      options: {
        customShellPath: '',
        ssh: makeSshOptions(),
        windowsPty: { useConpty: true },
      },
      cmd,
    });

    expect(shell).toEqual({
      id: 'zsh',
      binPath: '/bin/zsh',
      binName: 'zsh',
      friendlyName: 'zsh',
    });
    expect(creationStatus).toBe(PtyProcessCreationStatus.ShellNotResolved);
  });

  it("Teleport Connect env variables are prepended to the user's WSLENV for wsl.exe", async () => {
    const cmd: ShellCommand = {
      kind: 'pty.shell',
      clusterName: 'bar',
      proxyHost: 'baz',
      shellId: 'wsl.exe',
    };

    const { processOptions } = await buildPtyOptions({
      settings: makeRuntimeSettings({
        platform: 'win32',
        availableShells: [
          {
            id: 'wsl.exe',
            binName: 'wsl.exe',
            friendlyName: '',
            binPath: '',
          },
        ],
      }),
      options: {
        customShellPath: '',
        ssh: makeSshOptions(),
        windowsPty: { useConpty: true },
      },
      cmd,
      processEnv: {
        // Simulate the user defined WSLENV var.
        WSLENV: 'CUSTOM_VAR',
      },
    });

    expect(processOptions.env.WSLENV).toBe(
      'CUSTOM_VAR:KUBECONFIG/p:TERM_PROGRAM:TERM_PROGRAM_VERSION:TELEPORT_CLUSTER:TELEPORT_PROXY:TELEPORT_HOME/p:TELEPORT_TOOLS_VERSION'
    );
  });
});
