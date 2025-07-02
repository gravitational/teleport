/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import EventEmitter from 'events';

import { DeepLinkParseResult } from 'teleterm/deepLinks';
import { AgentProcessState } from 'teleterm/mainProcess/types';
// createConfigService has to be imported directly from configService.ts.
// teleterm/services/config/index.ts reexports the config service client which depends on electron.
// Importing electron breaks the fixtures if that's done from within storybook.
import { createConfigService } from 'teleterm/services/config/configService';
import { createMockFileStorage } from 'teleterm/services/fileStorage/fixtures/mocks';
import { MainProcessClient, RuntimeSettings } from 'teleterm/types';

export class MockMainProcessClient implements MainProcessClient {
  configService: ReturnType<typeof createConfigService>;
  private events: EventEmitter<{ 'deep-link-launch': [DeepLinkParseResult] }>;
  private frontendAppInit: {
    promise: Promise<void>;
    resolve: () => void;
    reject: (error: Error) => void;
  };

  constructor(private runtimeSettings: Partial<RuntimeSettings> = {}) {
    this.configService = createConfigService({
      configFile: createMockFileStorage(),
      jsonSchemaFile: createMockFileStorage(),
      settings: this.getRuntimeSettings(),
    });
    this.events = new EventEmitter();
    // Mirrors the implementation of frontendAppInit in WindowsManager.
    this.frontendAppInit = {
      promise: undefined,
      resolve: undefined,
      reject: undefined,
    };
    this.frontendAppInit.promise = new Promise((resolve, reject) => {
      this.frontendAppInit.resolve = resolve;
      this.frontendAppInit.reject = reject;
    });
  }

  subscribeToNativeThemeUpdate() {
    return { cleanup: () => undefined };
  }

  subscribeToAgentUpdate() {
    return { cleanup: () => undefined };
  }

  subscribeToDeepLinkLaunch(listener: (res: DeepLinkParseResult) => void) {
    this.events.addListener('deep-link-launch', listener);
    return {
      cleanup: () => this.events.removeListener('deep-link-launch', listener),
    };
  }

  launchDeepLink(res: DeepLinkParseResult) {
    this.events.emit('deep-link-launch', res);
  }

  getRuntimeSettings(): RuntimeSettings {
    return makeRuntimeSettings(this.runtimeSettings);
  }

  getResolvedChildProcessAddresses = () =>
    Promise.resolve({
      tsh: '',
      shared: '',
    });

  openTerminalContextMenu() {}

  openClusterContextMenu() {}

  openTabContextMenu() {}

  showFileSaveDialog() {
    return Promise.resolve({
      canceled: false,
      filePath: '',
    });
  }

  saveTextToFile() {
    return Promise.resolve({
      canceled: false,
    });
  }

  fileStorage = createMockFileStorage();

  removeKubeConfig(): Promise<void> {
    return Promise.resolve(undefined);
  }

  async forceFocusWindow() {}

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

  async downloadAgent() {}

  async verifyAgent() {}

  createAgentConfigFile() {
    return Promise.resolve();
  }

  isAgentConfigFileCreated() {
    return Promise.resolve(false);
  }

  openAgentLogsDirectory() {
    return Promise.resolve();
  }

  killAgent(): Promise<void> {
    return Promise.resolve();
  }

  runAgent(): Promise<void> {
    return Promise.resolve();
  }

  getAgentState(): AgentProcessState {
    return { status: 'not-started' };
  }

  getAgentLogs(): string {
    return '';
  }

  async removeAgentDirectory() {}

  async tryRemoveConnectMyComputerAgentBinary() {}

  signalUserInterfaceReadiness() {
    this.frontendAppInit.resolve();
  }

  /** Mirrors the implementation of whenFrontendAppIsReady in WindowsManager. */
  whenFrontendAppIsReady(): Promise<void> {
    return this.frontendAppInit.promise;
  }

  refreshClusterList() {}

  async selectDirectoryForDesktopSession() {
    return '';
  }
}

export const makeRuntimeSettings = (
  runtimeSettings?: Partial<RuntimeSettings>
): RuntimeSettings => ({
  platform: 'darwin' as const,
  dev: true,
  debug: true,
  insecure: true,
  userDataDir: '',
  sessionDataDir: '',
  tempDataDir: '',
  agentBinaryPath: '',
  binDir: '',
  certsDir: '',
  kubeConfigsDir: '',
  logsDir: '',
  defaultOsShellId: 'zsh',
  availableShells: [
    { id: 'zsh', friendlyName: 'zsh', binPath: '/bin/zsh', binName: 'zsh' },
  ],
  tshd: {
    requestedNetworkAddress: '',
    binaryPath: '',
    homeDir: '',
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
  // Should be kept in sync with the default proxyVersion of makeRootCluster.
  appVersion: '11.1.0',
  isLocalBuild: runtimeSettings?.appVersion === '1.0.0-dev',
  username: 'alice',
  hostname: 'staging-mac-mini',
  ...runtimeSettings,
});
