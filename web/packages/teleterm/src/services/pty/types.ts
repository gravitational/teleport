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
  initCommand?: string;
};

export type TshLoginCommand = PtyCommandBase & {
  kind: 'pty.tsh-login';
  login?: string;
  serverId: string;
  rootClusterId: string;
  leafClusterId?: string;
};

export type TshKubeLoginCommand = PtyCommandBase & {
  kind: 'pty.tsh-kube-login';
  kubeId: string;
  kubeConfigRelativePath: string;
  rootClusterId: string;
  leafClusterId?: string;
};

type PtyCommandBase = {
  proxyHost: string;
  clusterName: string;
};

export type PtyCommand = ShellCommand | TshLoginCommand | TshKubeLoginCommand;
