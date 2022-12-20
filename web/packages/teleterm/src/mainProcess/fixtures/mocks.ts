import { RuntimeSettings, MainProcessClient } from 'teleterm/types';
import { createMockFileStorage } from 'teleterm/services/fileStorage/fixtures/mocks';
import { createMockConfigService } from 'teleterm/services/config/fixtures/mocks';

const platform = 'darwin';

export class MockMainProcessClient implements MainProcessClient {
  getRuntimeSettings(): RuntimeSettings {
    return {
      platform,
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

  configService = createMockConfigService({});

  fileStorage = createMockFileStorage();

  removeKubeConfig(): Promise<void> {
    return Promise.resolve(undefined);
  }

  forceFocusWindow() {}
}
