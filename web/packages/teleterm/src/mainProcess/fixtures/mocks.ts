import { RuntimeSettings, MainProcessClient } from 'teleterm/types';
import { createMockFileStorage } from 'teleterm/services/fileStorage/fixtures/mocks';
// createConfigService has to be imported directly from configService.ts.
// teleterm/services/config/index.ts reexports the config service client which depends on electron.
// Importing electron breaks the fixtures if that's done from within storybook.
import { createConfigService } from 'teleterm/services/config/configService';

export class MockMainProcessClient implements MainProcessClient {
  configService: ReturnType<typeof createConfigService>;

  constructor(private runtimeSettings: Partial<RuntimeSettings> = {}) {
    this.configService = createConfigService(
      createMockFileStorage(),
      this.getRuntimeSettings().platform
    );
  }

  getRuntimeSettings(): RuntimeSettings {
    return { ...defaultRuntimeSettings, ...this.runtimeSettings };
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

const defaultRuntimeSettings = {
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
};
