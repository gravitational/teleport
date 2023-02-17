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
