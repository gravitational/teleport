export type PtyProcessOptions = {
  env: Record<string, string>;
  path: string;
  args: string[];
  cwd?: string;
  initCommand?: string;
};

export type IPtyProcess = {
  write(data: string): void;
  resize(cols: number, rows: number): void;
  dispose(): void;
  onData(cb: (data: string) => void): void;
  onOpen(cb: () => void): void;
  start(cols: number, rows: number): void;
  onExit(cb: (ev: { exitCode: number; signal?: number }) => void): void;
  getCwd(): Promise<string>;
};
