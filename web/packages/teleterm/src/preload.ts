import path from 'path';
import { contextBridge } from 'electron';
import createTshClient from 'teleterm/services/tshd/createClient';
import createPtyService from 'teleterm/services/pty/ptyService';
import createMainProcessClient from 'teleterm/mainProcess/mainProcessClient';
import createLoggerService from 'teleterm/services/logger';
import PreloadLogger from 'teleterm/logger';
import { ElectronGlobals } from './types';
import { createFileStorage } from 'teleterm/services/fileStorage';

const mainProcessClient = createMainProcessClient();
const runtimeSettings = mainProcessClient.getRuntimeSettings();
const loggerService = createLoggerService({
  dev: runtimeSettings.dev,
  dir: runtimeSettings.userDataDir,
});

PreloadLogger.init(loggerService);

const fileStorage = createFileStorage({
  filePath: path.join(runtimeSettings.userDataDir, 'app_state.json'),
});

const tshClient = createTshClient(runtimeSettings.tshd.networkAddr);
const ptyServiceClient = createPtyService(runtimeSettings);

contextBridge.exposeInMainWorld('electron', {
  mainProcessClient,
  tshClient,
  ptyServiceClient,
  loggerService,
  fileStorage,
} as ElectronGlobals);
