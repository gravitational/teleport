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

import { ChildProcess, fork, spawn, exec } from 'child_process';
import path from 'path';
import fs from 'fs/promises';

import { promisify } from 'util';

import {
  app,
  dialog,
  ipcMain,
  Menu,
  MenuItemConstructorOptions,
  nativeTheme,
  shell,
} from 'electron';

import { FileStorage, RuntimeSettings } from 'teleterm/types';
import { subscribeToFileStorageEvents } from 'teleterm/services/fileStorage';
import {
  LoggerColor,
  KeepLastChunks,
  createFileLoggerService,
} from 'teleterm/services/logger';
import {
  ChildProcessAddresses,
  MainProcessIpc,
  RendererIpc,
} from 'teleterm/mainProcess/types';
import { getAssetPath } from 'teleterm/mainProcess/runtimeSettings';
import { RootClusterUri } from 'teleterm/ui/uri';
import Logger from 'teleterm/logger';
import {
  TSH_AUTOUPDATE_ENV_VAR,
  TSH_AUTOUPDATE_OFF,
} from 'teleterm/node/tshAutoupdate';

import {
  ConfigService,
  subscribeToConfigServiceEvents,
} from '../services/config';

import { subscribeToTerminalContextMenuEvent } from './contextMenus/terminalContextMenu';
import { subscribeToTabContextMenuEvent } from './contextMenus/tabContextMenu';
import { resolveNetworkAddress, ResolveError } from './resolveNetworkAddress';
import { WindowsManager } from './windowsManager';
import { downloadAgent, verifyAgent, FileDownloader } from './agentDownloader';
import {
  createAgentConfigFile,
  isAgentConfigFileCreated,
  removeAgentDirectory,
  generateAgentConfigPaths,
} from './createAgentConfigFile';
import { AgentRunner } from './agentRunner';
import { terminateWithTimeout } from './terminateWithTimeout';

import type { CreateAgentConfigFileArgs } from './createAgentConfigFile';

type Options = {
  settings: RuntimeSettings;
  logger: Logger;
  configService: ConfigService;
  appStateFileStorage: FileStorage;
  configFileStorage: FileStorage;
  windowsManager: WindowsManager;
};

export default class MainProcess {
  readonly settings: RuntimeSettings;
  private readonly logger: Logger;
  private readonly configService: ConfigService;
  private tshdProcess: ChildProcess;
  private tshdProcessLastLogs: KeepLastChunks<string>;
  private sharedProcess: ChildProcess;
  private sharedProcessLastLogs: KeepLastChunks<string>;
  private appStateFileStorage: FileStorage;
  private configFileStorage: FileStorage;
  private resolvedChildProcessAddresses: Promise<ChildProcessAddresses>;
  private windowsManager: WindowsManager;
  // this function can be safely called concurrently
  private downloadAgentShared = sharePromise(() =>
    downloadAgent(
      new FileDownloader(this.windowsManager.getWindow()),
      this.settings,
      process.env
    )
  );
  private readonly agentRunner: AgentRunner;

  private constructor(opts: Options) {
    this.settings = opts.settings;
    this.logger = opts.logger;
    this.configService = opts.configService;
    this.appStateFileStorage = opts.appStateFileStorage;
    this.configFileStorage = opts.configFileStorage;
    this.windowsManager = opts.windowsManager;
    this.agentRunner = new AgentRunner(
      this.settings,
      path.join(__dirname, 'agentCleanupDaemon.js'),
      (rootClusterUri, state) => {
        const window = this.windowsManager.getWindow();
        if (window.isDestroyed()) {
          return;
        }
        window.webContents.send(
          RendererIpc.ConnectMyComputerAgentUpdate,
          rootClusterUri,
          state
        );
      }
    );
  }

  static create(opts: Options) {
    const instance = new MainProcess(opts);
    instance._init();
    return instance;
  }

  async dispose(): Promise<void> {
    this.windowsManager.dispose();
    await Promise.all([
      // sending usage events on tshd shutdown has 10-seconds timeout
      terminateWithTimeout(this.tshdProcess, 10_000, () => {
        this.gracefullyKillTshdProcess();
      }),
      terminateWithTimeout(this.sharedProcess),
      this.agentRunner.killAll(),
    ]);
  }

  private _init() {
    this.updateAboutPanelIfNeeded();
    this._setAppMenu();
    try {
      this._initTshd();
      this._initSharedProcess();
      this._initResolvingChildProcessAddresses();
      this._initIpc();
    } catch (err) {
      this.logger.error('Failed to start main process: ', err.message);
      app.exit(1);
    }
  }

  private _initTshd() {
    const { binaryPath, flags, homeDir } = this.settings.tshd;
    this.logger.info(`Starting tsh daemon from ${binaryPath}`);

    this.tshdProcess = spawn(binaryPath, flags, {
      stdio: 'pipe', // stdio must be set to `pipe` as the gRPC server address is read from stdout
      windowsHide: true,
      env: {
        ...process.env,
        TELEPORT_HOME: homeDir,
        [TSH_AUTOUPDATE_ENV_VAR]: TSH_AUTOUPDATE_OFF,
      },
    });

    this.logProcessExitAndError('tshd', this.tshdProcess);

    const loggerService = createFileLoggerService({
      dev: this.settings.dev,
      dir: this.settings.logsDir,
      name: TSHD_LOGGER_NAME,
      loggerNameColor: LoggerColor.Cyan,
      passThroughMode: true,
    });

    this.tshdProcessLastLogs = new KeepLastChunks(NO_OF_LAST_LOGS_KEPT);
    loggerService.pipeProcessOutputIntoLogger(
      this.tshdProcess,
      this.tshdProcessLastLogs
    );
  }

  private _initSharedProcess() {
    this.sharedProcess = fork(
      path.join(__dirname, 'sharedProcess.js'),
      [`--runtimeSettingsJson=${JSON.stringify(this.settings)}`],
      {
        stdio: 'pipe', // stdio must be set to `pipe` as the gRPC server address is read from stdout
      }
    );

    this.logProcessExitAndError('shared process', this.sharedProcess);

    const loggerService = createFileLoggerService({
      dev: this.settings.dev,
      dir: this.settings.logsDir,
      name: SHARED_PROCESS_LOGGER_NAME,
      loggerNameColor: LoggerColor.Yellow,
      passThroughMode: true,
    });

    this.sharedProcessLastLogs = new KeepLastChunks(NO_OF_LAST_LOGS_KEPT);
    loggerService.pipeProcessOutputIntoLogger(
      this.sharedProcess,
      this.sharedProcessLastLogs
    );
  }

  private _initResolvingChildProcessAddresses(): void {
    this.resolvedChildProcessAddresses = Promise.all([
      resolveNetworkAddress(
        this.settings.tshd.requestedNetworkAddress,
        this.tshdProcess
      ).catch(
        rewrapResolveError(
          this.logger,
          this.settings,
          'the tsh daemon',
          TSHD_LOGGER_NAME,
          this.tshdProcessLastLogs
        )
      ),
      resolveNetworkAddress(
        this.settings.sharedProcess.requestedNetworkAddress,
        this.sharedProcess
      ).catch(
        rewrapResolveError(
          this.logger,
          this.settings,
          'the shared helper process',
          SHARED_PROCESS_LOGGER_NAME,
          this.sharedProcessLastLogs
        )
      ),
    ]).then(([tsh, shared]) => ({ tsh, shared }));
  }

  private _initIpc() {
    ipcMain.on(MainProcessIpc.GetRuntimeSettings, event => {
      event.returnValue = this.settings;
    });

    ipcMain.on('main-process-should-use-dark-colors', event => {
      event.returnValue = nativeTheme.shouldUseDarkColors;
    });

    ipcMain.handle('main-process-get-resolved-child-process-addresses', () => {
      return this.resolvedChildProcessAddresses;
    });

    // the handler can remove a single kube config file or entire directory for given cluster
    ipcMain.handle(
      'main-process-remove-kube-config',
      (
        _,
        options: {
          relativePath: string;
          isDirectory?: boolean;
        }
      ) => {
        const { kubeConfigsDir } = this.settings;
        const filePath = path.join(kubeConfigsDir, options.relativePath);
        const isOutOfRoot = filePath.indexOf(kubeConfigsDir) !== 0;

        if (isOutOfRoot) {
          return Promise.reject('Invalid path');
        }
        return fs
          .rm(filePath, { recursive: !!options.isDirectory })
          .catch(error => {
            if (error.code !== 'ENOENT') {
              throw error;
            }
          });
      }
    );

    ipcMain.handle('main-process-show-file-save-dialog', (_, filePath) =>
      dialog.showSaveDialog({
        defaultPath: path.basename(filePath),
      })
    );

    ipcMain.handle('main-process-force-focus-window', () => {
      this.windowsManager.forceFocusWindow();
    });

    // Used in the `tsh install` command on macOS to make the bundled tsh available in PATH.
    // Returns true if tsh got successfully installed, false if the user closed the osascript
    // prompt. Throws an error when osascript fails.
    ipcMain.handle('main-process-symlink-tsh-macos', async () => {
      const source = this.settings.tshd.binaryPath;
      const target = '/usr/local/bin/tsh';
      const prompt =
        'Teleport Connect wants to create a symlink for tsh in /usr/local/bin.';
      const command = `osascript -e "do shell script \\"mkdir -p /usr/local/bin && ln -sf '${source}' '${target}'\\" with prompt \\"${prompt}\\" with administrator privileges"`;

      try {
        await promisify(exec)(command);
        this.logger.info(`Created the symlink to ${source} under ${target}`);
        return true;
      } catch (error) {
        // Ignore the error if the user canceled the prompt.
        // https://developer.apple.com/library/archive/documentation/AppleScript/Conceptual/AppleScriptLangGuide/reference/ASLR_error_codes.html#//apple_ref/doc/uid/TP40000983-CH220-SW2
        if (error instanceof Error && error.message.includes('-128')) {
          return false;
        }
        this.logger.error(error);
        throw error;
      }
    });

    ipcMain.handle('main-process-remove-tsh-symlink-macos', async () => {
      const target = '/usr/local/bin/tsh';
      const prompt =
        'Teleport Connect wants to remove a symlink for tsh from /usr/local/bin.';
      const command = `osascript -e "do shell script \\"rm '${target}'\\" with prompt \\"${prompt}\\" with administrator privileges"`;

      try {
        await promisify(exec)(command);
        this.logger.info(`Removed the symlink under ${target}`);
        return true;
      } catch (error) {
        // Ignore the error if the user canceled the prompt.
        // https://developer.apple.com/library/archive/documentation/AppleScript/Conceptual/AppleScriptLangGuide/reference/ASLR_error_codes.html#//apple_ref/doc/uid/TP40000983-CH220-SW2
        if (error instanceof Error && error.message.includes('-128')) {
          return false;
        }
        this.logger.error(error);
        throw error;
      }
    });

    ipcMain.handle('main-process-open-config-file', async () => {
      const path = this.configFileStorage.getFilePath();
      await shell.openPath(path);
      return path;
    });

    ipcMain.handle(MainProcessIpc.DownloadConnectMyComputerAgent, () =>
      this.downloadAgentShared()
    );

    ipcMain.handle(MainProcessIpc.VerifyConnectMyComputerAgent, async () => {
      await verifyAgent(this.settings.agentBinaryPath);
    });

    ipcMain.handle(
      'main-process-connect-my-computer-create-agent-config-file',
      (_, args: CreateAgentConfigFileArgs) =>
        createAgentConfigFile(this.settings, {
          proxy: args.proxy,
          token: args.token,
          rootClusterUri: args.rootClusterUri,
          username: args.username,
        })
    );

    ipcMain.handle(
      'main-process-connect-my-computer-is-agent-config-file-created',
      async (
        _,
        args: {
          rootClusterUri: RootClusterUri;
        }
      ) => isAgentConfigFileCreated(this.settings, args.rootClusterUri)
    );

    ipcMain.handle(
      'main-process-connect-my-computer-kill-agent',
      async (
        _,
        args: {
          rootClusterUri: RootClusterUri;
        }
      ) => {
        await this.agentRunner.kill(args.rootClusterUri);
      }
    );

    ipcMain.handle(
      'main-process-connect-my-computer-remove-agent-directory',
      (
        _,
        args: {
          rootClusterUri: RootClusterUri;
        }
      ) => removeAgentDirectory(this.settings, args.rootClusterUri)
    );

    ipcMain.handle(MainProcessIpc.TryRemoveConnectMyComputerAgentBinary, () =>
      this.agentRunner.tryRemoveAgentBinary()
    );

    ipcMain.handle(
      'main-process-connect-my-computer-run-agent',
      async (
        _,
        args: {
          rootClusterUri: RootClusterUri;
        }
      ) => {
        await this.agentRunner.start(args.rootClusterUri);
      }
    );

    ipcMain.on(
      'main-process-connect-my-computer-get-agent-state',
      (
        event,
        args: {
          rootClusterUri: RootClusterUri;
        }
      ) => {
        event.returnValue = this.agentRunner.getState(args.rootClusterUri);
      }
    );

    ipcMain.on(
      'main-process-connect-my-computer-get-agent-logs',
      (
        event,
        args: {
          rootClusterUri: RootClusterUri;
        }
      ) => {
        event.returnValue = this.agentRunner.getLogs(args.rootClusterUri);
      }
    );

    ipcMain.handle(
      'main-process-open-agent-logs-directory',
      async (
        _,
        args: {
          rootClusterUri: RootClusterUri;
        }
      ) => {
        const { logsDirectory } = generateAgentConfigPaths(
          this.settings,
          args.rootClusterUri
        );
        const error = await shell.openPath(logsDirectory);
        if (error) {
          throw new Error(error);
        }
      }
    );

    subscribeToTerminalContextMenuEvent();
    subscribeToTabContextMenuEvent();
    subscribeToConfigServiceEvents(this.configService);
    subscribeToFileStorageEvents(this.appStateFileStorage);
  }

  private _setAppMenu() {
    const isMac = this.settings.platform === 'darwin';
    const commonHelpTemplate: MenuItemConstructorOptions[] = [
      { label: 'Open Documentation', click: openDocsUrl },
      {
        label: 'Open Logs Directory',
        click: () => openLogsDirectory(this.settings),
      },
    ];

    // Enable actions like reload or toggle dev tools only in debug mode.
    const viewMenuTemplate: MenuItemConstructorOptions = this.settings.debug
      ? { role: 'viewMenu' }
      : {
          label: 'View',
          submenu: [
            { role: 'resetZoom' },
            { role: 'zoomIn' },
            { role: 'zoomOut' },
            { type: 'separator' },
            { role: 'togglefullscreen' },
          ],
        };

    const macTemplate: MenuItemConstructorOptions[] = [
      { role: 'appMenu' },
      { role: 'editMenu' },
      viewMenuTemplate,
      {
        label: 'Window',
        submenu: [{ role: 'minimize' }, { role: 'zoom' }],
      },
      {
        role: 'help',
        submenu: commonHelpTemplate,
      },
    ];

    const otherTemplate: MenuItemConstructorOptions[] = [
      { role: 'fileMenu' },
      { role: 'editMenu' },
      viewMenuTemplate,
      { role: 'windowMenu' },
      {
        role: 'help',
        submenu: [
          ...commonHelpTemplate,
          { type: 'separator' },
          { role: 'about' },
        ],
      },
    ];

    const menu = Menu.buildFromTemplate(isMac ? macTemplate : otherTemplate);
    Menu.setApplicationMenu(menu);
  }

  private updateAboutPanelIfNeeded(): void {
    // On Windows and Linux default menu does not show copyrights.
    if (
      this.settings.platform === 'linux' ||
      this.settings.platform === 'win32'
    ) {
      app.setAboutPanelOptions({
        applicationName: app.getName(),
        applicationVersion: app.getVersion(),
        iconPath: getAssetPath('icon-linux/512x512.png'), //.ico is not supported
        copyright: `Copyright © ${new Date().getFullYear()} Gravitational, Inc.`,
      });
    }
  }

  /**
   * On Windows, where POSIX signals do not exist, the only way to gracefully
   * kill a process is to send Ctrl-Break to its console. This task is done by
   * `tsh daemon stop` program. On Unix, the standard `SIGTERM` signal is sent.
   */
  private gracefullyKillTshdProcess() {
    if (this.settings.platform !== 'win32') {
      this.tshdProcess.kill('SIGTERM');
      return;
    }

    const logger = new Logger('Daemon stop');
    const daemonStop = spawn(
      this.settings.tshd.binaryPath,
      ['daemon', 'stop', `--pid=${this.tshdProcess.pid}`],
      {
        windowsHide: true,
        timeout: 2_000,
      }
    );
    daemonStop.on('error', error => {
      logger.error('daemon stop process failed to start', error);
    });
    daemonStop.stderr.setEncoding('utf-8');
    daemonStop.stderr.on('data', logger.error);
  }

  private logProcessExitAndError(
    processName: string,
    childProcess: ChildProcess
  ) {
    childProcess.on('error', error => {
      this.logger.error(`${processName} failed to start`, error);
    });

    childProcess.once('exit', (code, signal) => {
      const codeOrSignal = [
        // code can be 0, so we cannot just check it the same way as the signal.
        code != null && `code ${code}`,
        signal && `signal ${signal}`,
      ]
        .filter(Boolean)
        .join(' ');

      this.logger.info(`${processName} exited with ${codeOrSignal}`);
    });
  }
}

const TSHD_LOGGER_NAME = 'tshd';
const SHARED_PROCESS_LOGGER_NAME = 'shared';
const DOCS_URL = 'https://goteleport.com/docs/use-teleport/teleport-connect/';

function openDocsUrl() {
  shell.openExternal(DOCS_URL);
}

function openLogsDirectory(settings: RuntimeSettings) {
  shell.openPath(settings.logsDir);
}

/** Shares promise returned from `promiseFn` across multiple concurrent callers. */
function sharePromise<T>(promiseFn: () => Promise<T>): () => Promise<T> {
  let pending: Promise<T> | undefined = undefined;

  return () => {
    if (!pending) {
      pending = promiseFn();
      pending.finally(() => {
        pending = undefined;
      });
    }
    return pending;
  };
}

// The number of lines was chosen by looking at logs from the shared process when the glibc version
// is too old and making sure that we'd have been able to see the actual issue in the error dialog.
// See the PR description for the logs: https://github.com/gravitational/teleport/pull/38724
const NO_OF_LAST_LOGS_KEPT = 25;

function rewrapResolveError(
  logger: Logger,
  runtimeSettings: RuntimeSettings,
  processName: string,
  processLoggerName: string,
  processLastLogs: KeepLastChunks<string>
) {
  return (error: unknown) => {
    if (!(error instanceof ResolveError)) {
      throw error;
    }

    // Log the original error for full address.
    logger.error(error);

    const logPath = path.join(
      runtimeSettings.logsDir,
      `${processLoggerName}.log`
    );
    // TODO(ravicious): It'd have been ideal to get the logs from the file instead of keeping last n
    // lines in memory.
    // We tried to use winston.Logger.prototype.query for that but it kept returning no results,
    // even with structured logging turned on.
    const lastLogs = processLastLogs.getChunks().join('\n');

    throw new Error(
      `Could not communicate with ${processName}.\n\n` +
        `Last logs from ${logPath}:\n${lastLogs}`
    );
  };
}
