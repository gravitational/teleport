import { RuntimeSettings, MainProcessClient } from 'teleterm/types';
import { ConfigService } from 'teleterm/services/config';
import { createMockFileStorage } from 'teleterm/services/fileStorage/fixtures/mocks';

export class MockMainProcessClient implements MainProcessClient {
  getRuntimeSettings(): RuntimeSettings {
    return {
      platform: 'darwin',
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
  }

  getResolvedChildProcessAddresses = () =>
    Promise.resolve({ tsh: '', shared: '' });

  openTerminalContextMenu() {}

  openClusterContextMenu() {}

  openTabContextMenu() {}

  showFileSaveDialog() {
    return Promise.resolve({ canceled: false, filePath: '' });
  }

  configService = {
    get: () => ({
      keyboardShortcuts: {},
      appearance: {
        fonts: {},
      },
    }),
    update: () => undefined,
  } as unknown as ConfigService;

  fileStorage = createMockFileStorage();

  removeKubeConfig(): Promise<void> {
    return Promise.resolve(undefined);
  }

  forceFocusWindow() {}
}
