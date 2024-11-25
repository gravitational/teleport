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

import { makeCustomShellFromPath, Shell } from 'teleterm/mainProcess/shell';
import { RuntimeSettings } from 'teleterm/mainProcess/types';
import {
  TSH_AUTOUPDATE_ENV_VAR,
  TSH_AUTOUPDATE_OFF,
} from 'teleterm/node/tshAutoupdate';
import { CUSTOM_SHELL_ID } from 'teleterm/services/config/appConfigSchema';
import { PtyProcessOptions } from 'teleterm/sharedProcess/ptyHost';
import { assertUnreachable } from 'teleterm/ui/utils';

import {
  PtyCommand,
  PtyProcessCreationStatus,
  ShellCommand,
  SshOptions,
  TshKubeLoginCommand,
  WindowsPty,
} from '../types';
import {
  resolveShellEnvCached,
  ResolveShellEnvTimeoutError,
} from './resolveShellEnv';

type PtyOptions = {
  ssh: SshOptions;
  windowsPty: Pick<WindowsPty, 'useConpty'>;
  customShellPath: string;
};

const WSLENV_VAR = 'WSLENV';

export async function buildPtyOptions({
  settings,
  options,
  cmd,
  processEnv = process.env,
}: {
  settings: RuntimeSettings;
  options: PtyOptions;
  cmd: PtyCommand;
  processEnv?: typeof process.env;
}): Promise<{
  processOptions: PtyProcessOptions;
  shell: Shell;
  creationStatus: PtyProcessCreationStatus;
}> {
  const defaultShell = settings.availableShells.find(
    s => s.id === settings.defaultOsShellId
  );
  let shell = defaultShell;
  let failedToResolveShell = false;

  if (cmd.kind === 'pty.shell') {
    const resolvedShell = await resolveShell(cmd, settings, options);
    if (!resolvedShell) {
      failedToResolveShell = true;
    } else {
      shell = resolvedShell;
    }
  }

  return resolveShellEnvCached(shell.binPath)
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
      // combinedEnv is going to be used as env by every command coming out of buildPtyOptions. Some
      // commands might add extra variables, but they shouldn't remove any of the env vars that are
      // added here.
      const combinedEnv = {
        ...processEnv,
        ...shellEnv,
        TERM_PROGRAM: 'Teleport_Connect',
        TERM_PROGRAM_VERSION: settings.appVersion,
        TELEPORT_HOME: settings.tshd.homeDir,
        TELEPORT_CLUSTER: cmd.clusterName,
        TELEPORT_PROXY: cmd.proxyHost,
        [TSH_AUTOUPDATE_ENV_VAR]: TSH_AUTOUPDATE_OFF,
      };

      // The regular env vars are not available in WSL,
      // they need to be passed via the special variable WSLENV.
      // Note that path variables have /p postfix which translates the paths from Win32 to WSL.
      // https://devblogs.microsoft.com/commandline/share-environment-vars-between-wsl-and-windows/
      if (settings.platform === 'win32' && shell.binName === 'wsl.exe') {
        const wslEnv = [
          'KUBECONFIG/p',
          'TERM_PROGRAM',
          'TERM_PROGRAM_VERSION',
          'TELEPORT_CLUSTER',
          'TELEPORT_PROXY',
          'TELEPORT_HOME/p',
          TSH_AUTOUPDATE_ENV_VAR,
        ];
        // Preserve the user defined WSLENV and add ours (ours takes precedence).
        combinedEnv[WSLENV_VAR] = [combinedEnv[WSLENV_VAR], wslEnv]
          .flat()
          .join(':');
      }

      return {
        processOptions: getPtyProcessOptions({
          settings: settings,
          options: options,
          cmd: cmd,
          env: combinedEnv,
          shellBinPath: shell.binPath,
        }),
        shell,
        creationStatus: failedToResolveShell
          ? PtyProcessCreationStatus.ShellNotResolved
          : creationStatus,
      };
    });
}

export function getPtyProcessOptions({
  settings,
  options,
  cmd,
  env,
  shellBinPath,
}: {
  settings: RuntimeSettings;
  options: PtyOptions;
  cmd: PtyCommand;
  env: typeof process.env;
  shellBinPath: string;
}): PtyProcessOptions {
  const useConpty = options.windowsPty?.useConpty;

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
        path: shellBinPath,
        args: [],
        cwd: cmd.cwd,
        env: { ...env, ...cmd.env },
        initMessage: cmd.initMessage,
        useConpty,
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
        path: shellBinPath,
        args: isWindows ? powershellCommandArgs : bashCommandArgs,
        env: { ...env, KUBECONFIG: getKubeConfigFilePath(cmd, settings) },
        useConpty,
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
        ...(options.ssh.forwardAgent ? ['--forward-agent'] : []),
        loginHost,
      ];

      return {
        path: settings.tshd.binaryPath,
        args,
        env,
        useConpty,
      };
    }

    case 'pty.gateway-cli-client': {
      // TODO(ravicious): Set argv0 when node-pty adds support for it.
      // https://github.com/microsoft/node-pty/issues/472
      return {
        path: cmd.path,
        args: cmd.args,
        env: { ...env, ...cmd.env },
        useConpty,
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
  //
  // For process.env on Windows, Path and PATH are the same (case insensitivity).
  // Node.js have special setters and getters, so no matter what property you set,
  // the single underlying value is updated. However, since we merge many sources
  // of env vars into a single object with the object spread (let env = { ...process.env }),
  // theses setters and getters are lost.
  // The problem happens when user variables and system variables use different
  // casing for PATH and Node.js merges them into a single variable, and we have
  // to figure out its casing.
  // vscode does it the same way.
  const pathKey = getPropertyCaseInsensitive(env, 'PATH');
  env[pathKey] = [settings.binDir, env[pathKey]]
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

async function resolveShell(
  cmd: ShellCommand,
  settings: RuntimeSettings,
  ptyOptions: PtyOptions
): Promise<Shell | undefined> {
  if (cmd.shellId !== CUSTOM_SHELL_ID) {
    return settings.availableShells.find(s => s.id === cmd.shellId);
  }

  const { customShellPath } = ptyOptions;
  if (customShellPath) {
    return makeCustomShellFromPath(customShellPath);
  }
}

function getPropertyCaseInsensitive(
  env: Record<string, string>,
  key: string
): string | undefined {
  const pathKeys = Object.keys(env).filter(
    k => k.toLowerCase() === key.toLowerCase()
  );
  return pathKeys.length > 0 ? pathKeys[0] : key;
}
