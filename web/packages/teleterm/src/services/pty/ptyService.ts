import { RuntimeSettings } from 'teleterm/types';
import PtyProcess, { TermEventEnum } from './ptyProcess';
import { PtyCommand, PtyOptions, PtyServiceClient } from './types';
import { resolveShellEnvCached } from './resolveShellEnv';

export default function createPtyService(
  settings: RuntimeSettings
): PtyServiceClient {
  return {
    async createPtyProcess(cmd: PtyCommand) {
      const options = await buildOptions(settings, cmd);
      const _ptyProcess = new PtyProcess(options);

      return {
        start(cols: number, rows: number) {
          _ptyProcess.start(cols, rows);
        },

        write(data: string) {
          _ptyProcess.send(data);
        },

        resize(cols: number, rows: number) {
          _ptyProcess.resize(cols, rows);
        },

        dispose() {
          _ptyProcess.dispose();
        },

        onData(cb: (data: string) => void) {
          _ptyProcess.addListener(TermEventEnum.DATA, cb);
        },

        onOpen(cb: () => void) {
          _ptyProcess.addListener(TermEventEnum.OPEN, cb);
        },

        getStatus() {
          return _ptyProcess.getStatus();
        },

        getPid() {
          return _ptyProcess.getPid();
        },

        getCwd() {
          return _ptyProcess.getCwd();
        },

        onExit(cb: (ev: { exitCode: number; signal?: number }) => void) {
          _ptyProcess.addListener(TermEventEnum.EXIT, cb);
        },
      };
    },
  };
}

async function buildOptions(
  settings: RuntimeSettings,
  cmd: PtyCommand
): Promise<PtyOptions> {
  const env = {
    ...process.env,
    ...(await resolveShellEnvCached(settings.defaultShell)),
    TELEPORT_HOME: settings.tshd.homeDir,
    TELEPORT_CLUSTER: cmd.actualClusterName,
    TELEPORT_PROXY: cmd.proxyHost,
  };

  switch (cmd.kind) {
    case 'pty.shell':
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
        env,
        initCommand: cmd.initCommand,
      };

    case 'pty.tsh-kube-login':
      return {
        //path: settings.tshd.binaryPath,
        path: settings.defaultShell,
        args: [
          `-c`,
          `${settings.tshd.binaryPath}`,
          `--proxy=${cmd.rootClusterId}`,
          `kube`,
          `login`,
          `${cmd.kubeId}`,
        ],
        env,
      };

    case 'pty.tsh-login':
      const loginHost = cmd.login
        ? `${cmd.login}@${cmd.serverId}`
        : cmd.serverId;

      return {
        path: settings.tshd.binaryPath,
        args: [`--proxy=${cmd.rootClusterId}`, 'ssh', loginHost],
        env,
      };
    default:
      throw Error(`Unknown pty command: ${cmd}`);
  }
}

function prependBinDirToPath(
  env: typeof process.env,
  settings: RuntimeSettings
) {
  let path: string = env['PATH'] || '';

  if (!path.trim()) {
    path = settings.binDir;
  } else {
    path = settings.binDir + ':' + path;
  }

  env['PATH'] = path;
}
