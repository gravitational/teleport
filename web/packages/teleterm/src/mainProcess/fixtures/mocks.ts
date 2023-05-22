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
import { ConfigService } from 'teleterm/services/config';
import { createMockFileStorage } from 'teleterm/services/fileStorage/fixtures/mocks';
import { keyboardShortcutsConfigProvider } from 'teleterm/services/config/providers/keyboardShortcutsConfigProvider';

export class MockMainProcessClient implements MainProcessClient {
  configService: ConfigService;

  constructor(private runtimeSettings: Partial<RuntimeSettings> = {}) {
    this.configService = {
      get: () => ({
        keyboardShortcuts: keyboardShortcutsConfigProvider.getDefaults(
          this.getRuntimeSettings().platform
        ),
        appearance: {
          fonts: {},
        },
      }),
      update: () => undefined,
    } as unknown as ConfigService;
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
  ...runtimeSettings,
});
