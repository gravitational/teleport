import { RuntimeSettings, MainProcessClient } from 'teleterm/types';
import { createMockFileStorage } from 'teleterm/services/fileStorage/fixtures/mocks';
// createConfigService has to be imported directly from configService.ts.
// teleterm/services/config/index.ts reexports the config service client which depends on electron.
// Importing electron breaks the fixtures if that's done from within storybook.
import { createConfigService } from 'teleterm/services/config/configService';

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
      installationId: '123e4567-e89b-12d3-a456-426614174000',
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

  configService = createConfigService(createMockFileStorage(), platform);

  fileStorage = createMockFileStorage();

  removeKubeConfig(): Promise<void> {
    return Promise.resolve(undefined);
  }

  forceFocusWindow() {}
}
