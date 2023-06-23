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

import { RuntimeSettings, MainProcessClient } from 'teleterm/types';
import { createMockFileStorage } from 'teleterm/services/fileStorage/fixtures/mocks';
// createConfigService has to be imported directly from configService.ts.
// teleterm/services/config/index.ts reexports the config service client which depends on electron.
// Importing electron breaks the fixtures if that's done from within storybook.
import { createConfigService } from 'teleterm/services/config/configService';

export class MockMainProcessClient implements MainProcessClient {
  configService: ReturnType<typeof createConfigService>;

  constructor(private runtimeSettings: Partial<RuntimeSettings> = {}) {
    this.configService = createConfigService({
      configFile: createMockFileStorage(),
      jsonSchemaFile: createMockFileStorage(),
      platform: this.getRuntimeSettings().platform,
    });
  }

  getRuntimeSettings(): RuntimeSettings {
    return makeRuntimeSettings(this.runtimeSettings);
  }

  getResolvedChildProcessAddresses = () =>
    Promise.resolve({ tsh: '', shared: '' });

  openTerminalContextMenu() {}

  openClusterContextMenu() {}

  openTabContextMenu() {}

  showFileSaveDialog() {
    return Promise.resolve({ canceled: false, filePath: '' });
  }

  fileStorage = createMockFileStorage();

  removeKubeConfig(): Promise<void> {
    return Promise.resolve(undefined);
  }

  forceFocusWindow() {}

  async symlinkTshMacOs() {
    return true;
  }

  async removeTshSymlinkMacOs() {
    return true;
  }

  async openConfigFile() {
    return '';
  }

  shouldUseDarkColors() {
    return true;
  }

  subscribeToNativeThemeUpdate() {
    return { cleanup: () => undefined };
  }
  subscribeToAgentStart() {
    return { cleanup: () => undefined };
  }
}

export const makeRuntimeSettings = (
  runtimeSettings?: Partial<RuntimeSettings>
): RuntimeSettings => ({
  platform: 'darwin' as const,
  dev: true,
  userDataDir: '',
  binDir: '',
  certsDir: '',
  kubeConfigsDir: '',
  defaultShell: '',
  tshd: {
    insecure: true,
    requestedNetworkAddress: '',
    binaryPath: '',
    homeDir: '',
    flags: [],
  },
  sharedProcess: {
    requestedNetworkAddress: '',
  },
  tshdEvents: {
    requestedNetworkAddress: '',
  },
  installationId: '123e4567-e89b-12d3-a456-426614174000',
  arch: 'arm64',
  osVersion: '22.2.0',
  appVersion: '11.1.0',
  ...runtimeSettings,
});
