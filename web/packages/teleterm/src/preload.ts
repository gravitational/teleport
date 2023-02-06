import { contextBridge } from 'electron';

import createTshClient from 'teleterm/services/tshd/createClient';
import createMainProcessClient from 'teleterm/mainProcess/mainProcessClient';
import createLoggerService from 'teleterm/services/logger';
import PreloadLogger from 'teleterm/logger';

import { createPtyService } from 'teleterm/services/pty/ptyService';
import { getClientCredentials } from 'teleterm/services/grpcCredentials';

import { ElectronGlobals } from './types';

const mainProcessClient = createMainProcessClient();
const runtimeSettings = mainProcessClient.getRuntimeSettings();
const loggerService = createLoggerService({
  dev: runtimeSettings.dev,
  dir: runtimeSettings.userDataDir,
  name: 'renderer',
});

PreloadLogger.init(loggerService);

contextBridge.exposeInMainWorld('electron', getElectronGlobals());

async function getElectronGlobals(): Promise<ElectronGlobals> {
  const [addresses, credentials] = await Promise.all([
    mainProcessClient.getResolvedChildProcessAddresses(),
    getClientCredentials(runtimeSettings),
  ]);
  const tshClient = createTshClient(addresses.tsh, credentials.tsh);
  const ptyServiceClient = createPtyService(
    addresses.shared,
    credentials.shared,
    runtimeSettings
  );

  return {
    mainProcessClient,
    tshClient,
    ptyServiceClient,
    loggerService,
  };
}
