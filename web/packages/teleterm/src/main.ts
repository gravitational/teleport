import { spawn } from 'child_process';
import { app, globalShortcut } from 'electron';
import MainProcess from 'teleterm/mainProcess';
import { getRuntimeSettings } from 'teleterm/mainProcess/runtimeSettings';
import createLoggerService from 'teleterm/services/logger';
import Logger from 'teleterm/logger';
import * as types from 'teleterm/types';
import { ConfigServiceImpl } from 'teleterm/services/config';

const settings = getRuntimeSettings();
const logger = initMainLogger(settings);
const configService = new ConfigServiceImpl();

process.on('uncaughtException', error => {
  logger.error('', error);
  throw error;
});

// init main process
const mainProcess = MainProcess.create({ settings, logger, configService });

// node-pty is not yet context aware
app.allowRendererProcessReuse = false;
app.commandLine.appendSwitch('ignore-certificate-errors', 'true');

app.on('will-quit', () => {
  globalShortcut.unregisterAll();
  mainProcess.dispose();
});

app.whenReady().then(() => {
  if (mainProcess.settings.dev) {
    // allow restarts on F6
    globalShortcut.register('F6', () => {
      mainProcess.dispose();
      const [bin, ...args] = process.argv;
      const child = spawn(bin, args, {
        env: process.env,
        detached: true,
        stdio: 'inherit',
      });
      child.unref();
      app.exit();
    });
  }

  mainProcess.createWindow();
});

// Limit navigation capabilities to reduce the attack surface.
// See TEL-Q122-19 from "Teleport Core Testing Q1 2022" security audit.
//
// See also points 12, 13 and 14 from the Electron's security tutorial.
// https://github.com/electron/electron/blob/v17.2.0/docs/tutorial/security.md#12-verify-webview-options-before-creation
app.on('web-contents-created', (_, contents) => {
  contents.on('will-navigate', (event, navigationUrl) => {
    logger.warn(`Navigation to ${navigationUrl} blocked by 'will-navigate'`);
    event.preventDefault();
  });

  // The usage of webview is blocked by default, but let's include the handler just in case.
  // https://github.com/electron/electron/blob/v17.2.0/docs/api/webview-tag.md#enabling
  contents.on('will-attach-webview', (event, _, params) => {
    logger.warn(
      `Opening a webview to ${params.src} blocked by 'will-attach-webview'`
    );
    event.preventDefault();
  });

  contents.setWindowOpenHandler(({ url }) => {
    logger.warn(
      `Opening a new window to ${url} blocked by 'setWindowOpenHandler'`
    );
    return { action: 'deny' };
  });
});

function initMainLogger(settings: types.RuntimeSettings) {
  const service = createLoggerService({
    dev: settings.dev,
    dir: settings.userDataDir,
  });

  Logger.init(service);

  return new Logger('Main');
}
