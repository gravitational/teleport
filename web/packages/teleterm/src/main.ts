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

function initMainLogger(settings: types.RuntimeSettings) {
  const service = createLoggerService({
    dev: settings.dev,
    dir: settings.userDataDir,
  });

  Logger.init(service);

  return new Logger('Main');
}
