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

import path, { delimiter } from 'path';

import { RuntimeSettings } from 'teleterm/mainProcess/types';
import { PtyProcessOptions } from 'teleterm/sharedProcess/ptyHost';
import { assertUnreachable } from 'teleterm/ui/utils';

import {
  PtyCommand,
  PtyProcessCreationStatus,
  PtyOptions,
  TshKubeLoginCommand,
} from '../types';

import {
  resolveShellEnvCached,
  ResolveShellEnvTimeoutError,
} from './resolveShellEnv';

export async function buildPtyOptions(
  settings: RuntimeSettings,
  options: PtyOptions,
  cmd: PtyCommand
): Promise<{
  processOptions: PtyProcessOptions;
  creationStatus: PtyProcessCreationStatus;
}> {
  return resolveShellEnvCached(settings.defaultShell)
    .then(resolvedEnv => ({
      shellEnv: resolvedEnv,
      creationStatus: PtyProcessCreationStatus.Ok,
    }))
    .catch(error => {
      if (error instanceof ResolveShellEnvTimeoutError) {
        return {
          shellEnv: undefined,
          creationStatus: PtyProcessCreationStatus.ResolveShellEnvTimeout,
        };
      }
      throw error;
    })
    .then(({ shellEnv, creationStatus }) => {
      const combinedEnv = {
        ...process.env,
        ...shellEnv,
        TELEPORT_HOME: settings.tshd.homeDir,
        TELEPORT_CLUSTER: cmd.clusterName,
        TELEPORT_PROXY: cmd.proxyHost,
      };

      return {
        processOptions: getPtyProcessOptions(
          settings,
          options,
          cmd,
          combinedEnv
        ),
        creationStatus,
      };
    });
}

export function getPtyProcessOptions(
  settings: RuntimeSettings,
  options: PtyOptions,
  cmd: PtyCommand,
  env: typeof process.env
): PtyProcessOptions {
  switch (cmd.kind) {
    case 'pty.shell': {
      // Teleport Connect bundles a tsh binary, but the user might have one already on their system.
      // Since we use our own TELEPORT_HOME which might differ in format with the version that the
      // user has installed, let's prepend our bin directory to PATH.
      //
      // At the moment, this won't ensure that our bin dir is at the front of the path. When the
      // shell session starts, the shell will read the rc files. This means that if the user
      // prepends the path there, they can possibly have different version of tsh there.
      //
      // settings.binDir is present only in the packaged version of the app.
      if (settings.binDir) {
        prependBinDirToPath(env, settings);
      }

      return {
        path: settings.defaultShell,
        args: [],
        cwd: cmd.cwd,
        env: { ...env, ...cmd.env },
        initMessage: cmd.initMessage,
        useConpty: options.terminal.useConpty,
      };
    }

    case 'pty.tsh-kube-login': {
      const isWindows = settings.platform === 'win32';

      // backtick (PowerShell) and backslash (Bash) are used to escape a whitespace
      const escapedBinaryPath = settings.tshd.binaryPath.replaceAll(
        ' ',
        isWindows ? '` ' : '\\ '
      );
      const kubeLoginCommand = [
        escapedBinaryPath,
        `--proxy=${cmd.rootClusterId}`,
        `kube login ${cmd.kubeId} --cluster=${cmd.clusterName}`,
        settings.insecure && '--insecure',
      ]
        .filter(Boolean)
        .join(' ');
      const bashCommandArgs = ['-c', `${kubeLoginCommand};$SHELL`];
      const powershellCommandArgs = ['-NoExit', '-c', kubeLoginCommand];
      return {
        path: settings.defaultShell,
        args: isWindows ? powershellCommandArgs : bashCommandArgs,
        env: { ...env, KUBECONFIG: getKubeConfigFilePath(cmd, settings) },
        useConpty: options.terminal.useConpty,
      };
    }

    case 'pty.tsh-login': {
      const loginHost = cmd.login
        ? `${cmd.login}@${cmd.serverId}`
        : cmd.serverId;

      const args = [
        `--proxy=${cmd.rootClusterId}`,
        'ssh',
        ...(options.ssh.noResume ? ['--no-resume'] : []),
        '--forward-agent',
        loginHost,
      ];

      return {
        path: settings.tshd.binaryPath,
        args,
        env,
        useConpty: options.terminal.useConpty,
      };
    }

    case 'pty.gateway-cli-client': {
      // TODO(ravicious): Set argv0 when node-pty adds support for it.
      // https://github.com/microsoft/node-pty/issues/472
      return {
        path: cmd.path,
        args: cmd.args,
        env: { ...env, ...cmd.env },
        useConpty: options.terminal.useConpty,
      };
    }

    default:
      assertUnreachable(cmd);
  }
}

function prependBinDirToPath(
  env: typeof process.env,
  settings: RuntimeSettings
): void {
  // On Windows, if settings.binDir is already in Path, this function will simply put in the front,
  // guaranteeing that any shell session started from within Connect will use the bundled tsh.
  //
  // Windows seems to construct Path by first taking the system Path env var and adding to it the
  // user Path env var.
  const pathName = settings.platform === 'win32' ? 'Path' : 'PATH';
  env[pathName] = [settings.binDir, env[pathName]]
    .map(path => path?.trim())
    .filter(Boolean)
    .join(delimiter);
}

function getKubeConfigFilePath(
  command: TshKubeLoginCommand,
  settings: RuntimeSettings
): string {
  return path.join(settings.kubeConfigsDir, command.kubeConfigRelativePath);
}
