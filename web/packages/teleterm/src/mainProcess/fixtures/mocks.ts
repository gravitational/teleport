import { RuntimeSettings, MainProcessClient } from 'teleterm/types';
import { ConfigService } from 'teleterm/services/config';
import { createMockFileStorage } from 'teleterm/services/fileStorage/fixtures/mocks';

export class MockMainProcessClient implements MainProcessClient {
  getRuntimeSettings(): RuntimeSettings {
    return {
      platform: 'darwin',
      dev: true,
      userDataDir: '',
      defaultShell: '',
      tshd: {
        insecure: true,
        networkAddr: '',
        binaryPath: '',
        homeDir: '',
        flags: [],
      },
    };
  }

  openTerminalContextMenu() {}

  openClusterContextMenu() {}

  openTabContextMenu() {}

  configService = {
    get: () => ({
      keyboardShortcuts: {},
      appearance: {
        fonts: {},
      },
    }),
    update: () => undefined,
  } as unknown as ConfigService;

  fileStorage = createMockFileStorage()
}
