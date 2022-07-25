import { ChildProcess, fork, spawn } from 'child_process';

import path from 'path';

import { app, ipcMain, Menu, MenuItemConstructorOptions } from 'electron';

import { FileStorage, Logger, RuntimeSettings } from 'teleterm/types';

import { subscribeToFileStorageEvents } from 'teleterm/services/fileStorage';

import createLoggerService from 'teleterm/services/logger';

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
      this._initIpc();
    } catch (err) {
      this.logger.error('Failed to start main process: ', err.message);
      app.exit(1);
    }
  }

  private _initTshd() {
    const { binaryPath, flags, homeDir } = this.settings.tshd;
    this.tshdProcess = spawn(binaryPath, flags, {
      stdio: [null, 'pipe', 'pipe'],
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
        stdio: 'inherit',
      }
    );

    this.sharedProcess.on('error', error => {
      this.logger.error('shared process failed to start', error);
    });

    this.sharedProcess.once('exit', code => {
      this.logger.info('shared process exited with code:', code);
    });
  }

  private _initIpc() {
    ipcMain.on('main-process-get-runtime-settings', event => {
      event.returnValue = this.settings;
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
