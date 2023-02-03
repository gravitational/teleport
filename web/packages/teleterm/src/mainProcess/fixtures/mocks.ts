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
};
