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

import path, { delimiter } from 'path';

import { RuntimeSettings } from 'teleterm/mainProcess/types';
import { PtyProcessOptions } from 'teleterm/sharedProcess/ptyHost';
import { assertUnreachable } from 'teleterm/ui/utils';
import {
  TSH_AUTOUPDATE_ENV_VAR,
  TSH_AUTOUPDATE_OFF,
} from 'teleterm/node/tshAutoupdate';

import {
  PtyCommand,
  PtyProcessCreationStatus,
  TshKubeLoginCommand,
} from '../types';

import {
  resolveShellEnvCached,
  ResolveShellEnvTimeoutError,
} from './resolveShellEnv';

export async function buildPtyOptions(
  settings: RuntimeSettings,
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
      // combinedEnv is going to be used as env by every command coming out of buildPtyOptions. Some
      // commands might add extra variables, but they shouldn't remove any of the env vars that are
      // added here.
      const combinedEnv = {
        ...process.env,
        ...shellEnv,
        TERM_PROGRAM: 'Teleport_Connect',
        TERM_PROGRAM_VERSION: settings.appVersion,
        TELEPORT_HOME: settings.tshd.homeDir,
        TELEPORT_CLUSTER: cmd.clusterName,
        TELEPORT_PROXY: cmd.proxyHost,
        [TSH_AUTOUPDATE_ENV_VAR]: TSH_AUTOUPDATE_OFF,
      };

      return {
        processOptions: getPtyProcessOptions(settings, cmd, combinedEnv),
        creationStatus,
      };
    });
}

export function getPtyProcessOptions(
  settings: RuntimeSettings,
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
      };
    }

    case 'pty.tsh-login': {
      const loginHost = cmd.login
        ? `${cmd.login}@${cmd.serverId}`
        : cmd.serverId;

      return {
        path: settings.tshd.binaryPath,
        args: [
          `--proxy=${cmd.rootClusterId}`,
          'ssh',
          '--forward-agent',
          loginHost,
        ],
        env,
      };
    }

    case 'pty.gateway-cli-client': {
      // TODO(ravicious): Set argv0 when node-pty adds support for it.
      // https://github.com/microsoft/node-pty/issues/472
      return {
        path: cmd.path,
        args: cmd.args,
        env: { ...env, ...cmd.env },
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
