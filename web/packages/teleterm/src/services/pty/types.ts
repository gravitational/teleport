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

import { PtyProcessOptions, IPtyProcess } from 'teleterm/sharedProcess/ptyHost';

import { PtyEventsStreamHandler } from './ptyHost/ptyEventsStreamHandler';

export enum PtyProcessCreationStatus {
  Ok = 'Ok',
  ResolveShellEnvTimeout = 'ResolveShellEnvTimeout',
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
  }>;
};

export type ShellCommand = PtyCommandBase & {
  kind: 'pty.shell';
  cwd?: string;
  // env is a record of additional env variables that need to be set for the shell terminal and it
  // will be merged with process env.
  env?: Record<string, string>;
  helpMsg?: string;
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

export type TshKubeLoginCommand = PtyCommandBase & {
  kind: 'pty.tsh-kube-login';
  kubeId: string;
  kubeConfigRelativePath: string;
  rootClusterId: string;
  leafClusterId?: string;
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
  | TshKubeLoginCommand
  | GatewayCliClientCommand;
