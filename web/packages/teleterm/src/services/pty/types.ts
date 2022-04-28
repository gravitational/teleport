export type PtyOptions = {
  env?: { [key: string]: string };
  path: string;
  args: string[] | string;
  cwd?: string;
  initCommand?: string;
};

export type PtyProcess = {
  write(data: string): void;
  resize(cols: number, rows: number): void;
  dispose(): void;
  onData(cb: (data: string) => void): void;
  onOpen(cb: () => void): void;
  start(cols: number, rows: number): void;
  onExit(cb: (ev: { exitCode: number; signal?: number }) => void);
  getPid(): number;
  getCwd(): Promise<string>;
};

export type PtyServiceClient = {
  createPtyProcess: (cmd: PtyCommand) => Promise<PtyProcess>;
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
  rootClusterId: string;
  leafClusterId?: string;
};

type PtyCommandBase = {
  proxyHost: string;
  actualClusterName: string;
}

export type PtyCommand = ShellCommand | TshLoginCommand | TshKubeLoginCommand;
