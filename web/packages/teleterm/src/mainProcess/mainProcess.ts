import { ChildProcess, fork, spawn } from 'child_process';

import path from 'path';

import { app, ipcMain, Menu, MenuItemConstructorOptions } from 'electron';

import { FileStorage, Logger, RuntimeSettings } from 'teleterm/types';

import { subscribeToFileStorageEvents } from 'teleterm/services/fileStorage';

import createLoggerService from 'teleterm/services/logger';
import { ChildProcessAddresses } from 'teleterm/mainProcess/types';

import {
  ConfigService,
  subscribeToConfigServiceEvents,
} from '../services/config';

import { subscribeToTerminalContextMenuEvent } from './contextMenus/terminalContextMenu';
import { subscribeToTabContextMenuEvent } from './contextMenus/tabContextMenu';

type Options = {
  settings: RuntimeSettings;
  logger: Logger;
  configService: ConfigService;
  fileStorage: FileStorage;
};

export default class MainProcess {
  readonly settings: RuntimeSettings;
  private readonly logger: Logger;
  private readonly configService: ConfigService;
  private tshdProcess: ChildProcess;
  private sharedProcess: ChildProcess;
  private fileStorage: FileStorage;
  private resolvedChildProcessAddresses: Promise<ChildProcessAddresses>;

  private constructor(opts: Options) {
    this.settings = opts.settings;
    this.logger = opts.logger;
    this.configService = opts.configService;
    this.fileStorage = opts.fileStorage;
  }

  static create(opts: Options) {
    const instance = new MainProcess(opts);
    instance._init();
    return instance;
  }

  dispose() {
    this.sharedProcess.kill('SIGTERM');
    this.tshdProcess.kill('SIGTERM');
  }

  private _init() {
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
    this.tshdProcess = spawn(binaryPath, flags, {
      stdio: 'pipe',
      windowsHide: true,
      env: {
        ...process.env,
        TELEPORT_HOME: homeDir,
      },
    });

    const tshdLogger = createLoggerService({
      dev: this.settings.dev,
      dir: this.settings.userDataDir,
      name: 'tshd',
      passThroughMode: true,
    });

    tshdLogger.pipeProcessOutputIntoLogger(this.tshdProcess.stdout);
    tshdLogger.pipeProcessOutputIntoLogger(this.tshdProcess.stderr);

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
        stdio: 'pipe',
      }
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
      this.resolveNetworkAddress(
        this.settings.tshd.requestedNetworkAddress,
        this.tshdProcess
      ),
      this.resolveNetworkAddress(
        this.settings.sharedProcess.requestedNetworkAddress,
        this.sharedProcess
      ),
    ]).then(([tsh, shared]) => ({ tsh, shared }));
  }

  private resolveNetworkAddress(
    requestedAddress: string,
    process: ChildProcess
  ): Promise<string> {
    if (new URL(requestedAddress).protocol === 'unix:') {
      return Promise.resolve(requestedAddress);
    }

    // TCP case
    return new Promise((resolve, reject) => {
      process.stdout.setEncoding('utf-8');
      let chunks = '';
      const timeout = setTimeout(() => {
        rejectOnError(
          new Error(
            `Could not resolve address (${requestedAddress}) for process ${process.spawnfile}. The operation timed out.`
          )
        );
      }, 10_000); // 10s

      const removeListeners = () => {
        process.stdout.off('data', findAddressInChunk);
        process.off('error', rejectOnError);
        clearTimeout(timeout);
      };

      const findAddressInChunk = (chunk: string) => {
        chunks += chunk;
        const matchResult = chunks.match(/\{CONNECT_GRPC_PORT:\s(\d+)}/);
        if (matchResult) {
          resolve(`localhost:${matchResult[1]}`);
          removeListeners();
        }
      };

      const rejectOnError = (error: Error) => {
        reject(error);
        removeListeners();
      };

      process.stdout.on('data', findAddressInChunk);
      process.on('error', rejectOnError);
    });
  }

  private _initIpc() {
    ipcMain.on('main-process-get-runtime-settings', event => {
      event.returnValue = this.settings;
    });

    ipcMain.handle('main-process-get-resolved-child-process-addresses', () => {
      return this.resolvedChildProcessAddresses;
    });

    subscribeToTerminalContextMenuEvent();
    subscribeToTabContextMenuEvent();
    subscribeToConfigServiceEvents(this.configService);
    subscribeToFileStorageEvents(this.fileStorage);
  }

  private _setAppMenu() {
    const isMac = this.settings.platform === 'darwin';

    const template: MenuItemConstructorOptions[] = [
      ...(isMac ? ([{ role: 'appMenu' }] as const) : []),
      ...(isMac ? [] : ([{ role: 'fileMenu' }] as const)),
      { role: 'editMenu' },
      { role: 'viewMenu' },
      isMac
        ? { role: 'windowMenu' }
        : {
            label: 'Window',
            submenu: [{ role: 'minimize' }, { role: 'zoom' }],
          },
      {
        role: 'help',
        submenu: [
          {
            label: 'Learn More',
            click: () => {},
          },
        ],
      },
    ];

    const menu = Menu.buildFromTemplate(template);
    Menu.setApplicationMenu(menu);
  }
}
