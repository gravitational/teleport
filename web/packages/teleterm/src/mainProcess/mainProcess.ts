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
  shell,
} from 'electron';
import { wait } from 'shared/utils/wait';

import { FileStorage, RuntimeSettings } from 'teleterm/types';
import { subscribeToFileStorageEvents } from 'teleterm/services/fileStorage';
import { LoggerColor, createFileLoggerService } from 'teleterm/services/logger';
import { ChildProcessAddresses } from 'teleterm/mainProcess/types';
import { getAssetPath } from 'teleterm/mainProcess/runtimeSettings';
import Logger from 'teleterm/logger';

import {
  ConfigService,
  subscribeToConfigServiceEvents,
} from '../services/config';

import { subscribeToTerminalContextMenuEvent } from './contextMenus/terminalContextMenu';
import { subscribeToTabContextMenuEvent } from './contextMenus/tabContextMenu';
import { resolveNetworkAddress } from './resolveNetworkAddress';
import { WindowsManager } from './windowsManager';

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
  private sharedProcess: ChildProcess;
  private appStateFileStorage: FileStorage;
  private configFileStorage: FileStorage;
  private resolvedChildProcessAddresses: Promise<ChildProcessAddresses>;
  private windowsManager: WindowsManager;

  private constructor(opts: Options) {
    this.settings = opts.settings;
    this.logger = opts.logger;
    this.configService = opts.configService;
    this.appStateFileStorage = opts.appStateFileStorage;
    this.configFileStorage = opts.configFileStorage;
    this.windowsManager = opts.windowsManager;
  }

  static create(opts: Options) {
    const instance = new MainProcess(opts);
    instance._init();
    return instance;
  }

  dispose() {
    this.killTshdProcess();
    this.sharedProcess.kill('SIGTERM');
    const processesExit = Promise.all([
      promisifyProcessExit(this.tshdProcess),
      promisifyProcessExit(this.sharedProcess),
    ]);
    // sending usage events on tshd shutdown has 10 seconds timeout
    const timeout = wait(10_000).then(() =>
      this.logger.error('Child process(es) did not exit within 10 seconds')
    );
    return Promise.race([processesExit, timeout]);
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
      },
    });

    const tshdPassThroughLogger = createFileLoggerService({
      dev: this.settings.dev,
      dir: this.settings.userDataDir,
      name: 'tshd',
      loggerNameColor: LoggerColor.Cyan,
      passThroughMode: true,
    });

    tshdPassThroughLogger.pipeProcessOutputIntoLogger(this.tshdProcess.stdout);
    tshdPassThroughLogger.pipeProcessOutputIntoLogger(this.tshdProcess.stderr);

    this.tshdProcess.on('error', error => {
      this.logger.error('tshd failed to start', error);
    });

    this.tshdProcess.once('exit', code => {
      this.logger.info('tshd exited with code:', code);
    });
  }

  private _initSharedProcess() {
    this.sharedProcess = fork(
      path.join(__dirname, 'sharedProcess.js'),
      [`--runtimeSettingsJson=${JSON.stringify(this.settings)}`],
      {
        stdio: 'pipe', // stdio must be set to `pipe` as the gRPC server address is read from stdout
      }
    );
    const sharedProcessPassThroughLogger = createFileLoggerService({
      dev: this.settings.dev,
      dir: this.settings.userDataDir,
      name: 'shared',
      loggerNameColor: LoggerColor.Yellow,
      passThroughMode: true,
    });

    sharedProcessPassThroughLogger.pipeProcessOutputIntoLogger(
      this.sharedProcess.stdout
    );
    sharedProcessPassThroughLogger.pipeProcessOutputIntoLogger(
      this.sharedProcess.stderr
    );

    this.sharedProcess.on('error', error => {
      this.logger.error('shared process failed to start', error);
    });

    this.sharedProcess.once('exit', code => {
      this.logger.info('shared process exited with code:', code);
    });
  }

  private _initResolvingChildProcessAddresses(): void {
    this.resolvedChildProcessAddresses = Promise.all([
      resolveNetworkAddress(
        this.settings.tshd.requestedNetworkAddress,
        this.tshdProcess
      ),
      resolveNetworkAddress(
        this.settings.sharedProcess.requestedNetworkAddress,
        this.sharedProcess
      ),
    ]).then(([tsh, shared]) => ({ tsh, shared }));
  }

  private _initIpc() {
    ipcMain.on('main-process-get-runtime-settings', event => {
      event.returnValue = this.settings;
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

    subscribeToTerminalContextMenuEvent();
    subscribeToTabContextMenuEvent();
    subscribeToConfigServiceEvents(this.configService);
    subscribeToFileStorageEvents(this.appStateFileStorage);
  }

  private _setAppMenu() {
    const isMac = this.settings.platform === 'darwin';

    // Enable actions like reload or toggle dev tools only in dev mode.
    const viewMenuTemplate: MenuItemConstructorOptions = this.settings.dev
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
        submenu: [{ label: 'Learn More', click: openDocsUrl }],
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
          { label: 'Learn More', click: openDocsUrl },
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
  private killTshdProcess() {
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
}

const DOCS_URL = 'https://goteleport.com/docs/use-teleport/teleport-connect/';

function openDocsUrl() {
  shell.openExternal(DOCS_URL);
}

function promisifyProcessExit(childProcess: ChildProcess) {
  return new Promise(resolve => childProcess.once('exit', resolve));
}
