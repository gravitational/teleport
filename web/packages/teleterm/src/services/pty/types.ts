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

import { Shell } from 'teleterm/mainProcess/shell';
import { IPtyProcess, PtyProcessOptions } from 'teleterm/sharedProcess/ptyHost';

import { PtyEventsStreamHandler } from './ptyHost/ptyEventsStreamHandler';

export enum PtyProcessCreationStatus {
  Ok = 'Ok',
  ResolveShellEnvTimeout = 'ResolveShellEnvTimeout',
  ShellNotResolved = 'ShellNotResolved',
}

export interface PtyHostClient {
  createPtyProcess(ptyOptions: PtyProcessOptions): Promise<string>;

  getCwd(ptyId: string): Promise<string>;

  exchangeEvents(ptyId: string): PtyEventsStreamHandler;
}

export type PtyServiceClient = {
  createPtyProcess: (cmd: PtyCommand) => Promise<{
    process: IPtyProcess;
    creationStatus: PtyProcessCreationStatus;
    windowsPty: WindowsPty;
    shell: Shell;
  }>;
};

/**
 * Pty information for Windows.
 * undefined for non-Windows OS.
 */
export type WindowsPty =
  | {
      useConpty: boolean;
      buildNumber: number;
    }
  | undefined;

export type ShellCommand = PtyCommandBase & {
  kind: 'pty.shell';
  cwd?: string;
  // env is a record of additional env variables that need to be set for the shell terminal and it
  // will be merged with process env.
  env?: Record<string, string>;
  // initMessage is a help message presented to the user at the beginning of
  // the shell to provide extra context.
  //
  // The initMessage is rendered on the terminal UI without being written or
  // read by the underlying PTY.
  initMessage?: string;
  /** Shell identifier. */
  shellId: string;
};

export type TshLoginCommand = PtyCommandBase & {
  kind: 'pty.tsh-login';
  // login is missing when the user executes `tsh ssh host` from the command bar without supplying
  // the login. In that case, login will be undefined and serverId will be equal to "host". tsh will
  // assume that login equals to the current OS user.
  login: string | undefined;
  serverId: string;
  rootClusterId: string;
  leafClusterId: string | undefined;
};

export type GatewayCliClientCommand = PtyCommandBase & {
  kind: 'pty.gateway-cli-client';
  // path is an absolute path to the CLI client. It is resolved on tshd side by GO's
  // os.exec.LookPath.
  //
  // It cannot be just the command name such as `psql` because Windows fails to resolve the
  // command name if it doesn't include the `.exe` suffix.
  path: string;
  // args is a Node.js-style list of arguments passed to the command, _without_ the command name as
  // the first element.
  args: string[];
  // env is a record of additional env variables that need to be set for the given CLI client. It
  // will be merged into process env before the client is started.
  env: Record<string, string>;
};

type PtyCommandBase = {
  proxyHost: string;
  clusterName: string;
};

export type PtyCommand =
  | ShellCommand
  | TshLoginCommand
  | GatewayCliClientCommand;

export type SshOptions = {
  /**
   * Disables SSH connection resumption when running `tsh ssh`
   * (by adding the `--no-resume` option).
   */
  noResume: boolean;
  /**
   * Enables agent forwarding when running `tsh ssh` by adding the --forward-agent option.
   */
  forwardAgent: boolean;
};

export type TerminalOptions = {
  windowsBackend: 'auto' | 'winpty';
};
